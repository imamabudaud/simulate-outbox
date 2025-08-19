package googleanalytics

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type AnalyticsEvent struct {
	Payload json.RawMessage `json:"payload"`
}

func Run(ctx context.Context, port string) error {
	e := echo.New()
	e.POST("/events", handleAnalyticsEvent)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: e,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	slog.Info("google analytics service started", "port", port)

	<-ctx.Done()
	return server.Shutdown(context.Background())
}

func handleAnalyticsEvent(c echo.Context) error {
	var event AnalyticsEvent
	if err := c.Bind(&event); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	var payload map[string]interface{}
	var orderID string
	if err := json.Unmarshal(event.Payload, &payload); err == nil {
		if id, ok := payload["orderId"].(string); ok {
			orderID = id
		}
	}

	slog.Info("analytics event received", "orderId", orderID, "payload", string(event.Payload), "timestamp", time.Now())
	return c.JSON(http.StatusOK, map[string]string{"status": "event processed successfully"})
}
