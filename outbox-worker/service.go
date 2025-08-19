package outboxworker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type OutboxMessage struct {
	ID         int        `json:"id"`
	Status     string     `json:"status"`
	Type       string     `json:"type"`
	Data       string     `json:"data"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
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

	_, err = db.Exec("DROP TABLE IF EXISTS outbox")
	if err != nil {
		return err
	}

	createOutboxTable := `
	CREATE TABLE outbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		status TEXT NOT NULL DEFAULT 'PENDING',
		type TEXT NOT NULL,
		data TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME
	);`

	_, err = db.Exec(createOutboxTable)
	return err
}

func Run(ctx context.Context, cronPeriod string) error {
	if err := initDB(); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer db.Close()

	cronPeriodInt, _ := strconv.Atoi(cronPeriod)
	ticker := time.NewTicker(time.Duration(cronPeriodInt) * time.Second)
	defer ticker.Stop()

	slog.Info("outbox worker started", "cron_period", cronPeriod)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			processOutboxMessages()
		}
	}
}

func processOutboxMessages() {
	slog.Info("processing outbox messages")

	countQuery := "SELECT COUNT(*) FROM outbox WHERE status = 'PENDING'"
	var count int
	err := db.QueryRow(countQuery).Scan(&count)
	if err != nil {
		slog.Error("failed to count pending outbox messages", "error", err)
		return
	}
	slog.Info("found pending outbox messages", "count", count)

	if count == 0 {
		slog.Info("no pending messages to process")
		return
	}

	slog.Info("sample message data:")
	sampleQuery := "SELECT id, type, data FROM outbox WHERE status = 'PENDING' LIMIT 1"
	var sampleID int
	var sampleType, sampleData string
	err = db.QueryRow(sampleQuery).Scan(&sampleID, &sampleType, &sampleData)
	if err != nil {
		slog.Error("failed to get sample message", "error", err)
	} else {
		slog.Info("sample message", "id", sampleID, "type", sampleType, "data", sampleData)
	}

	rows, err := db.Query("SELECT id, status, type, data, created_at FROM outbox WHERE status = 'PENDING'")
	if err != nil {
		slog.Error("failed to query pending outbox messages", "error", err)
		return
	}
	defer rows.Close()

	processedCount := 0
	for rows.Next() {
		var message OutboxMessage
		err := rows.Scan(&message.ID, &message.Status, &message.Type, &message.Data, &message.CreatedAt)
		if err != nil {
			slog.Error("failed to scan message", "error", err)
			continue
		}

		slog.Info("processing outbox message", "id", message.ID, "type", message.Type, "data", string(message.Data))

		if rand.Float32() < 0.3 {
			slog.Error("random failure occurred, message will be picked up later", "id", message.ID, "type", message.Type)
			continue
		}

		if err := processMessage(message); err != nil {
			slog.Error("failed to process outbox message", "id", message.ID, "type", message.Type, "error", err)
			continue
		}

		_, err = db.Exec("UPDATE outbox SET status = 'FINISHED', finished_at = CURRENT_TIMESTAMP WHERE id = ?", message.ID)
		if err != nil {
			slog.Error("failed to update outbox message status", "id", message.ID, "error", err)
		} else {
			processedCount++
			slog.Info("outbox message processed successfully", "id", message.ID, "type", message.Type)
		}
	}

	slog.Info("outbox processing completed", "total_found", count, "processed", processedCount)
}

func processMessage(message OutboxMessage) error {
	switch message.Type {
	case "EMAIL":
		return processEmailMessage(message)
	case "NOTIFY":
		return processNotificationMessage(message)
	case "ANALYTIC":
		return processAnalyticsMessage(message)
	default:
		return fmt.Errorf("unknown message type: %s", message.Type)
	}
}

func processEmailMessage(message OutboxMessage) error {
	var emailData map[string]interface{}
	if err := json.Unmarshal([]byte(message.Data), &emailData); err != nil {
		return fmt.Errorf("failed to unmarshal email data: %w", err)
	}

	slog.Info("processing email message", "recipients", emailData["recipients"], "subject", emailData["subject"])

	jsonData, _ := json.Marshal(emailData)
	resp, err := http.Post("http://localhost:8081/send-email", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to call email service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("email service returned status: %d", resp.StatusCode)
	}

	return nil
}

func processNotificationMessage(message OutboxMessage) error {
	var notificationData map[string]interface{}
	if err := json.Unmarshal([]byte(message.Data), &notificationData); err != nil {
		return fmt.Errorf("failed to unmarshal notification data: %w", err)
	}

	slog.Info("processing notification message", "deviceId", notificationData["deviceId"], "message", notificationData["message"])

	jsonData, _ := json.Marshal(notificationData)
	resp, err := http.Post("http://localhost:8082/send-notification", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to call notification service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification service returned status: %d", resp.StatusCode)
	}

	return nil
}

func processAnalyticsMessage(message OutboxMessage) error {
	var analyticsData map[string]interface{}
	if err := json.Unmarshal([]byte(message.Data), &analyticsData); err != nil {
		return fmt.Errorf("failed to unmarshal analytics data: %w", err)
	}

	slog.Info("processing analytics message", "event", analyticsData["event"], "orderId", analyticsData["orderId"])

	payloadData := map[string]interface{}{
		"payload": analyticsData,
	}

	jsonData, _ := json.Marshal(payloadData)
	resp, err := http.Post("http://localhost:9000/events", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to call google analytics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google analytics returned status: %d", resp.StatusCode)
	}

	return nil
}
