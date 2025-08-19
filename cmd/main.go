package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"substack-outbox/email-service"
	"substack-outbox/notification-service"
	"substack-outbox/google-analytics"
	"substack-outbox/order-basic"
	"substack-outbox/order-improved"
	"substack-outbox/outbox-worker"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found, using system environment variables")
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/main.go <service-name>")
		fmt.Println("Available services: email-service, notification-service, google-analytics, order-basic, order-improved, email-worker, notification-worker, outbox-worker")
		os.Exit(1)
	}

	serviceName := os.Args[1]
	viper.SetConfigName("env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Shutdown signal received")
		cancel()
	}()

	switch serviceName {
	case "email-service":
		emailservice.Run(ctx, viper.GetString("EMAIL_SERVICE_PORT"))
	case "notification-service":
		notificationservice.Run(ctx, viper.GetString("NOTIFICATION_SERVICE_PORT"))
	case "google-analytics":
		googleanalytics.Run(ctx, viper.GetString("GOOGLE_ANALYTICS_SERVICE_PORT"))
	case "order-basic":
		orderbasic.Run(ctx, viper.GetString("ORDER_BASIC_SERVICE_PORT"))
	case "order-improved":
		orderimproved.Run(ctx, viper.GetString("ORDER_IMPROVED_SERVICE_PORT"))
	case "email-worker":
		emailservice.RunWorker(ctx, viper.GetString("EMAIL_WORKER_CRON_PERIOD"))
	case "notification-worker":
		notificationservice.RunWorker(ctx, viper.GetString("NOTIFICATION_WORKER_CRON_PERIOD"))
	case "outbox-worker":
		outboxworker.Run(ctx, viper.GetString("OUTBOX_WORKER_CRON_PERIOD"))
	default:
		fmt.Printf("Unknown service: %s\n", serviceName)
		fmt.Println("Available services: email-service, notification-service, google-analytics, order-basic, order-improved, email-worker, notification-worker, outbox-worker")
		os.Exit(1)
	}
}
