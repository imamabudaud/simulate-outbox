package orderbasic

import (
	"bytes"
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

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./order_basic.db")
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

	createTable := `
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT NOT NULL,
		user_name TEXT NOT NULL,
		user_email TEXT NOT NULL,
		device_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'PENDING',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = db.Exec(createTable)
	return err
}

func Run(ctx context.Context, port string) error {
	if err := initDB(); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer db.Close()

	e := echo.New()
	e.POST("/finish-order", handleFinishOrder)
	e.GET("/orders", handleGetOrders)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: e,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	slog.Info("basic order service started", "port", port)

	<-ctx.Done()
	return server.Shutdown(context.Background())
}

func handleFinishOrder(c echo.Context) error {
	var req OrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	tx, err := db.Begin()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to start transaction"})
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO orders (order_id, user_name, user_email, device_id, status) VALUES (?, ?, ?, ?, ?)",
		req.OrderID, req.UserName, req.UserEmail, req.DeviceID, "PENDING",
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create order"})
	}

	if rand.Float32() < 0.3 {
		slog.Info("[ORDER-" + req.OrderID + "] random failure occurred during order processing")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "random failure occurred"})
	}
	slog.Info("[ORDER-" + req.OrderID + "] order created")

	_, err = tx.Exec("UPDATE orders SET status = 'FINISHED', updated_at = CURRENT_TIMESTAMP WHERE order_id = ?", req.OrderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update order status"})
	}
	slog.Info("[ORDER-" + req.OrderID + "] order updated")

	if err := callEmailService(req); err != nil {
		slog.Error("[ORDER-"+req.OrderID+"] failed to call email service", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to send email"})
	}
	slog.Info("[ORDER-" + req.OrderID + "] email service called")

	if err := callNotificationService(req); err != nil {
		slog.Error("[ORDER-"+req.OrderID+"] failed to call notification service", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to send notification"})
	}
	slog.Info("[ORDER-" + req.OrderID + "] notification service called")

	if err := callGoogleAnalytics(req); err != nil {
		slog.Error("[ORDER-"+req.OrderID+"] failed to call google analytics", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to send analytics"})
	}
	slog.Info("[ORDER-" + req.OrderID + "] google analytics called")

	if err := tx.Commit(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to commit transaction"})
	}

	slog.Info("[ORDER-" + req.OrderID + "] order finished successfully")
	return c.JSON(http.StatusOK, map[string]string{"status": "order finished successfully"})
}

func callEmailService(req OrderRequest) error {
	if rand.Float32() < 0.3 {
		return fmt.Errorf("random failure in email service")
	}

	emailData := map[string]interface{}{
		"recipients": []string{req.UserEmail},
		"subject":    "Order Completed",
		"body":       fmt.Sprintf("Your order %s has been completed successfully!", req.OrderID),
	}

	jsonData, _ := json.Marshal(emailData)
	resp, err := http.Post("http://localhost:8081/send-email", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("email service returned status: %d", resp.StatusCode)
	}

	return nil
}

func callNotificationService(req OrderRequest) error {
	if rand.Float32() < 0.3 {
		return fmt.Errorf("random failure in notification service")
	}

	notificationData := map[string]interface{}{
		"deviceId": []string{req.DeviceID},
		"message":  fmt.Sprintf("Order %s completed successfully!", req.OrderID),
	}

	jsonData, _ := json.Marshal(notificationData)
	resp, err := http.Post("http://localhost:8082/send-notification", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification service returned status: %d", resp.StatusCode)
	}

	return nil
}

func callGoogleAnalytics(req OrderRequest) error {
	if rand.Float32() < 0.3 {
		return fmt.Errorf("random failure in google analytics")
	}

	analyticsData := map[string]interface{}{
		"event":     "order_completed",
		"orderId":   req.OrderID,
		"userEmail": req.UserEmail,
		"timestamp": time.Now(),
	}

	payloadData := map[string]interface{}{
		"payload": analyticsData,
	}

	jsonData, _ := json.Marshal(payloadData)
	resp, err := http.Post("http://localhost:9000/events", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google analytics returned status: %d", resp.StatusCode)
	}

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
