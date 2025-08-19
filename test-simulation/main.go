package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

type OrderRequest struct {
	OrderID   string `json:"orderId"`
	UserName  string `json:"userName"`
	UserEmail string `json:"userEmail"`
	DeviceID  string `json:"deviceId"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run test-simulation/main.go <basic|improved> <order_count>")
		os.Exit(1)
	}

	mode := os.Args[1]
	orderCountStr := os.Args[2]

	orderCount, err := strconv.Atoi(orderCountStr)
	if err != nil {
		fmt.Printf("Invalid order count: %s\n", orderCountStr)
		os.Exit(1)
	}

	rand.Seed(time.Now().UnixNano())

	switch mode {
	case "basic":
		runBasicSimulation(orderCount)
	case "improved":
		runImprovedSimulation(orderCount)
	default:
		fmt.Printf("Unknown mode: %s. Use 'basic' or 'improved'\n", mode)
		os.Exit(1)
	}
}

func runBasicSimulation(orderCount int) {
	fmt.Printf("Running basic order simulation with %d orders...\n", orderCount)

	successCount := 0
	failureCount := 0

	for i := 0; i < orderCount; i++ {
		orderID := fmt.Sprintf("ORDER-%d", i+1)
		userName := fmt.Sprintf("User%d", i+1)
		userEmail := fmt.Sprintf("user%d@example.com", i+1)
		deviceID := fmt.Sprintf("DEVICE-%d", i+1)

		req := OrderRequest{
			OrderID:   orderID,
			UserName:  userName,
			UserEmail: userEmail,
			DeviceID:  deviceID,
		}

		if err := sendBasicOrder(req); err != nil {
			slog.Error("basic order failed", "orderId", orderID, "error", err)
			failureCount++
		} else {
			slog.Info("basic order succeeded", "orderId", orderID)
			successCount++
		}

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("Basic simulation completed. Success: %d, Failures: %d\n", successCount, failureCount)
}

func runImprovedSimulation(orderCount int) {
	fmt.Printf("Running improved order simulation with %d orders...\n", orderCount)

	successCount := 0
	failureCount := 0

	for i := 0; i < orderCount; i++ {
		orderID := fmt.Sprintf("ORDER-IMPROVED-%d", i+1)
		userName := fmt.Sprintf("User%d", i+1)
		userEmail := fmt.Sprintf("user%d@example.com", i+1)
		deviceID := fmt.Sprintf("DEVICE-%d", i+1)

		req := OrderRequest{
			OrderID:   orderID,
			UserName:  userName,
			UserEmail: userEmail,
			DeviceID:  deviceID,
		}

		if err := sendImprovedOrder(req); err != nil {
			slog.Error("improved order failed", "orderId", orderID, "error", err)
			failureCount++
		} else {
			slog.Info("improved order succeeded", "orderId", orderID)
			successCount++
		}

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("Improved simulation completed. Success: %d, Failures: %d\n", successCount, failureCount)
}

func sendBasicOrder(req OrderRequest) error {
	jsonData, _ := json.Marshal(req)
	resp, err := http.Post("http://localhost:8080/finish-order", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service returned status: %d", resp.StatusCode)
	}

	return nil
}

func sendImprovedOrder(req OrderRequest) error {
	jsonData, _ := json.Marshal(req)
	resp, err := http.Post("http://localhost:8083/finish-order-improved", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service returned status: %d", resp.StatusCode)
	}

	return nil
}
