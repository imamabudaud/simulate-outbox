package notificationservice

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

type NotificationRequest struct {
	DeviceID []string `json:"deviceId"`
	Message  string   `json:"message"`
}

type NotificationRecord struct {
	ID        int        `json:"id"`
	DeviceID  string     `json:"deviceId"`
	Message   string     `json:"message"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./notification_service.db")
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
	CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id TEXT NOT NULL,
		message TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'PENDING',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		sent_at DATETIME
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
	e.POST("/send-notification", handleSendNotification)
	e.GET("/notifications", handleGetNotifications)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: e,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	slog.Info("notification service started", "port", port)

	<-ctx.Done()
	return server.Shutdown(context.Background())
}

func handleSendNotification(c echo.Context) error {
	var req NotificationRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	deviceIDJSON, _ := json.Marshal(req.DeviceID)

	_, err := db.Exec(
		"INSERT INTO notifications (device_id, message, status) VALUES (?, ?, ?)",
		string(deviceIDJSON), req.Message, "PENDING",
	)
	if err != nil {
		slog.Error("failed to insert notification", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to store notification"})
	}

	slog.Info("notification stored", "deviceId", req.DeviceID, "message", req.Message)
	return c.JSON(http.StatusOK, map[string]string{"status": "notification stored successfully"})
}

func handleGetNotifications(c echo.Context) error {
	rows, err := db.Query("SELECT id, device_id, message, status, created_at, sent_at FROM notifications ORDER BY created_at DESC")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch notifications"})
	}
	defer rows.Close()

	var notifications []NotificationRecord
	for rows.Next() {
		var notification NotificationRecord
		var sentAt sql.NullTime
		err := rows.Scan(&notification.ID, &notification.DeviceID, &notification.Message, &notification.Status, &notification.CreatedAt, &sentAt)
		if err != nil {
			continue
		}
		if sentAt.Valid {
			notification.SentAt = &sentAt.Time
		}
		notifications = append(notifications, notification)
	}

	return c.JSON(http.StatusOK, notifications)
}

func RunWorker(ctx context.Context, cronPeriod string) error {
	if err := initDB(); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer db.Close()

	cronPeriodInt, _ := strconv.Atoi(cronPeriod)
	ticker := time.NewTicker(time.Duration(cronPeriodInt) * time.Second)
	defer ticker.Stop()

	slog.Info("notification worker started", "cron_period", cronPeriod)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			processPendingNotifications()
		}
	}
}

func processPendingNotifications() {
	slog.Info("processing pending notifications")

	countQuery := "SELECT COUNT(*) FROM notifications WHERE status = 'PENDING'"
	var count int
	err := db.QueryRow(countQuery).Scan(&count)
	if err != nil {
		slog.Error("failed to count pending notifications", "error", err)
		return
	}
	slog.Info("found pending notifications", "count", count)

	rows, err := db.Query("SELECT id, device_id, message FROM notifications WHERE status = 'PENDING'")
	if err != nil {
		slog.Error("failed to query pending notifications", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var notification NotificationRecord
		err := rows.Scan(&notification.ID, &notification.DeviceID, &notification.Message)
		if err != nil {
			continue
		}

		slog.Info("processing notification", "id", notification.ID, "deviceId", notification.DeviceID, "message", notification.Message)

		_, err = db.Exec("UPDATE notifications SET status = 'SENT', sent_at = CURRENT_TIMESTAMP WHERE id = ?", notification.ID)
		if err != nil {
			slog.Error("failed to update notification status", "id", notification.ID, "error", err)
		}
	}
}
