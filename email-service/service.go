package emailservice

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

type EmailRequest struct {
	Recipients []string `json:"recipients"`
	Subject    string   `json:"subject"`
	Body       string   `json:"body"`
}

type EmailRecord struct {
	ID         int        `json:"id"`
	Recipients string     `json:"recipients"`
	Subject    string     `json:"subject"`
	Body       string     `json:"body"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	SentAt     *time.Time `json:"sent_at,omitempty"`
}

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./email_service.db")
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
	CREATE TABLE IF NOT EXISTS emails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		recipients TEXT NOT NULL,
		subject TEXT NOT NULL,
		body TEXT NOT NULL,
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
	e.POST("/send-email", handleSendEmail)
	e.GET("/emails", handleGetEmails)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: e,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	slog.Info("email service started", "port", port)

	<-ctx.Done()
	return server.Shutdown(context.Background())
}

func handleSendEmail(c echo.Context) error {
	var req EmailRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	recipientsJSON, _ := json.Marshal(req.Recipients)

	_, err := db.Exec(
		"INSERT INTO emails (recipients, subject, body, status) VALUES (?, ?, ?, ?)",
		string(recipientsJSON), req.Subject, req.Body, "PENDING",
	)
	if err != nil {
		slog.Error("failed to insert email", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to store email"})
	}

	slog.Info("email stored", "recipients", req.Recipients, "subject", req.Subject)
	return c.JSON(http.StatusOK, map[string]string{"status": "email stored successfully"})
}

func handleGetEmails(c echo.Context) error {
	rows, err := db.Query("SELECT id, recipients, subject, body, status, created_at, sent_at FROM emails ORDER BY created_at DESC")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch emails"})
	}
	defer rows.Close()

	var emails []EmailRecord
	for rows.Next() {
		var email EmailRecord
		var sentAt sql.NullTime
		err := rows.Scan(&email.ID, &email.Recipients, &email.Subject, &email.Body, &email.Status, &email.CreatedAt, &sentAt)
		if err != nil {
			continue
		}
		if sentAt.Valid {
			email.SentAt = &sentAt.Time
		}
		emails = append(emails, email)
	}

	return c.JSON(http.StatusOK, emails)
}

func RunWorker(ctx context.Context, cronPeriod string) error {
	if err := initDB(); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer db.Close()

	cronPeriodInt, _ := strconv.Atoi(cronPeriod)
	ticker := time.NewTicker(time.Duration(cronPeriodInt) * time.Second)
	defer ticker.Stop()

	slog.Info("email worker started", "cron_period", cronPeriod)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			processPendingEmails()
		}
	}
}

func processPendingEmails() {
	slog.Info("processing pending emails")

	countQuery := "SELECT COUNT(*) FROM emails WHERE status = 'PENDING'"
	var count int
	err := db.QueryRow(countQuery).Scan(&count)
	if err != nil {
		slog.Error("failed to count pending emails", "error", err)
		return
	}
	slog.Info("found pending emails", "count", count)

	rows, err := db.Query("SELECT id, recipients, subject, body FROM emails WHERE status = 'PENDING'")
	if err != nil {
		slog.Error("failed to query pending emails", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var email EmailRecord
		err := rows.Scan(&email.ID, &email.Recipients, &email.Subject, &email.Body)
		if err != nil {
			continue
		}

		slog.Info("processing email", "id", email.ID, "recipients", email.Recipients, "subject", email.Subject, "body", email.Body)

		_, err = db.Exec("UPDATE emails SET status = 'SENT', sent_at = CURRENT_TIMESTAMP WHERE id = ?", email.ID)
		if err != nil {
			slog.Error("failed to update email status", "id", email.ID, "error", err)
		}
	}
}
