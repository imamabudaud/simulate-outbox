package orderimproved

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

type OrderRequest struct {
	OrderID   string `json:"orderId"`
	UserName  string `json:"userName"`
	UserEmail string `json:"userEmail"`
	DeviceID  string `json:"deviceId"`
}

type OrderRecord struct {
	ID        int       `json:"id"`
	OrderID   string    `json:"orderId"`
	UserName  string    `json:"userName"`
	UserEmail string    `json:"userEmail"`
	DeviceID  string    `json:"deviceId"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OutboxMessage struct {
	ID         int             `json:"id"`
	Status     string          `json:"status"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	CreatedAt  time.Time       `json:"created_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
}

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./order_improved.db")
	if err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA busy_timeout=5000")
	if err != nil {
		return err
	}

	_, err = db.Exec("DROP TABLE IF EXISTS orders")
	if err != nil {
		return err
	}

	_, err = db.Exec("DROP TABLE IF EXISTS outbox")
	if err != nil {
		return err
	}

	createOrdersTable := `
	CREATE TABLE orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT NOT NULL,
		user_name TEXT NOT NULL,
		user_email TEXT NOT NULL,
		device_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'PENDING',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	createOutboxTable := `
	CREATE TABLE outbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		status TEXT NOT NULL DEFAULT 'PENDING',
		type TEXT NOT NULL,
		data TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME
	);`

	_, err = db.Exec(createOrdersTable)
	if err != nil {
		return err
	}

	_, err = db.Exec(createOutboxTable)
	return err
}

func Run(ctx context.Context, port string) error {
	if err := initDB(); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer db.Close()

	e := echo.New()
	e.POST("/finish-order-improved", handleFinishOrder)
	e.GET("/orders", handleGetOrders)
	e.GET("/outbox", handleGetOutbox)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: e,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	slog.Info("improved order service started", "port", port)

	<-ctx.Done()
	return server.Shutdown(context.Background())
}

func handleFinishOrder(c echo.Context) error {
	var req OrderRequest
	if err := c.Bind(&req); err != nil {
		slog.Error("failed to bind request", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	slog.Info("processing order request", "orderId", req.OrderID, "userName", req.UserName, "userEmail", req.UserEmail, "deviceId", req.DeviceID)

	if rand.Float32() < 0.1 {
		slog.Info("random failure occurred during order processing", "orderId", req.OrderID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "random failure occurred"})
	}

	tx, err := db.Begin()
	if err != nil {
		slog.Error("failed to start transaction", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to start transaction"})
	}
	defer tx.Rollback()

	slog.Info("transaction started", "orderId", req.OrderID)

	_, err = tx.Exec(
		"INSERT INTO orders (order_id, user_name, user_email, device_id, status) VALUES (?, ?, ?, ?, ?)",
		req.OrderID, req.UserName, req.UserEmail, req.DeviceID, "PENDING",
	)
	if err != nil {
		slog.Error("failed to create order", "orderId", req.OrderID, "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create order"})
	}

	_, err = tx.Exec("UPDATE orders SET status = 'FINISHED', updated_at = CURRENT_TIMESTAMP WHERE order_id = ?", req.OrderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update order status"})
	}

	if err := createOutboxMessage(tx, "EMAIL", map[string]interface{}{
		"recipients": []string{req.UserEmail},
		"subject":    "Order Completed",
		"body":       fmt.Sprintf("Your order %s has been completed successfully!", req.OrderID),
	}); err != nil {
		slog.Error("failed to create email outbox message", "orderId", req.OrderID, "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create email outbox message"})
	}
	slog.Info("[ORDER-" + req.OrderID + "] email outbox message created")

	if err := createOutboxMessage(tx, "NOTIFY", map[string]interface{}{
		"deviceId": []string{req.DeviceID},
		"message":  fmt.Sprintf("Order %s completed successfully!", req.OrderID),
	}); err != nil {
		slog.Error("failed to create notification outbox message", "orderId", req.OrderID, "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create notification outbox message"})
	}
	slog.Info("[ORDER-" + req.OrderID + "] notification outbox message created")

	if err := createOutboxMessage(tx, "ANALYTIC", map[string]interface{}{
		"event":     "order_completed",
		"orderId":   req.OrderID,
		"userEmail": req.UserEmail,
		"timestamp": time.Now(),
	}); err != nil {
		slog.Error("failed to create analytics outbox message", "orderId", req.OrderID, "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create analytics outbox message"})
	}
	slog.Info("[ORDER-" + req.OrderID + "] analytics outbox message created")

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit transaction", "orderId", req.OrderID, "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to commit transaction"})
	}

	slog.Info("order finished successfully with outbox messages", "orderId", req.OrderID)
	return c.JSON(http.StatusOK, map[string]string{"status": "order finished successfully"})
}

func createOutboxMessage(tx *sql.Tx, messageType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	result, err := tx.Exec(
		"INSERT INTO outbox (status, type, data) VALUES (?, ?, ?)",
		"PENDING", messageType, string(jsonData),
	)
	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()
	slog.Info("outbox message inserted", "id", id, "type", messageType, "data", string(jsonData))
	return nil
}

func handleGetOrders(c echo.Context) error {
	rows, err := db.Query("SELECT id, order_id, user_name, user_email, device_id, status, created_at, updated_at FROM orders ORDER BY created_at DESC")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch orders"})
	}
	defer rows.Close()

	var orders []OrderRecord
	for rows.Next() {
		var order OrderRecord
		err := rows.Scan(&order.ID, &order.OrderID, &order.UserName, &order.UserEmail, &order.DeviceID, &order.Status, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			continue
		}
		orders = append(orders, order)
	}

	return c.JSON(http.StatusOK, orders)
}

func handleGetOutbox(c echo.Context) error {
	rows, err := db.Query("SELECT id, status, type, data, created_at, finished_at FROM outbox ORDER BY created_at DESC")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch outbox messages"})
	}
	defer rows.Close()

	var messages []OutboxMessage
	for rows.Next() {
		var message OutboxMessage
		var finishedAt sql.NullTime
		err := rows.Scan(&message.ID, &message.Status, &message.Type, &message.Data, &message.CreatedAt, &finishedAt)
		if err != nil {
			continue
		}
		if finishedAt.Valid {
			message.FinishedAt = &finishedAt.Time
		}
		messages = append(messages, message)
	}

	return c.JSON(http.StatusOK, messages)
}
