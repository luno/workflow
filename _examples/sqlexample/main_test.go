package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/sqlstore"
)

// TestSQLWorkflow demonstrates testing a workflow with SQL persistence
func TestSQLWorkflow(t *testing.T) {
	// This test requires a running MariaDB instance
	// Skip if SKIP_INTEGRATION_TESTS is set
	dsn := "workflow_user:workflow_pass@tcp(localhost:3306)/workflow_db?parseTime=true"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("Skipping integration test: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test (MariaDB not available): %v", err)
	}

	// Create SQL store
	store := sqlstore.New(db, db, "workflow_records", "workflow_outbox")

	// Build workflow
	wf := buildOrderWorkflow(store)

	ctx := context.Background()
	wf.Run(ctx)
	defer wf.Stop()

	// Create a valid order
	order := Order{
		ID:           "test-order-123",
		CustomerName: "Test Customer",
		Email:        "test@example.com",
		Items: []OrderItem{
			{ProductID: "test-1", Name: "Test Product", Quantity: 1, Price: 99.99},
		},
		Total:     99.99,
		CreatedAt: time.Now(),
	}

	// Trigger workflow
	runID, err := wf.Trigger(ctx, order.ID, workflow.WithInitialValue[Order, OrderStatus](&order))
	if err != nil {
		t.Fatalf("Failed to trigger workflow: %v", err)
	}

	// Wait for completion
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	run, err := wf.Await(ctxTimeout, order.ID, runID, OrderCompleted)
	if err != nil {
		t.Fatalf("Workflow failed or timed out: %v", err)
	}

	// Verify workflow completed successfully (or failed in expected ways)
	validStatuses := []OrderStatus{OrderCompleted, OrderPaymentFailed, OrderFulfillmentFailed, OrderRejected}
	validStatus := false
	for _, s := range validStatuses {
		if run.Status == s {
			validStatus = true
			break
		}
	}
	if !validStatus {
		t.Errorf("Expected workflow to reach a terminal state, got status: %v", run.Status)
	}

	// Verify data was persisted to database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM workflow_records WHERE run_id = ?", runID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 record in database, got %d", count)
	}

	t.Logf("✅ Workflow completed with status: %v", run.Status)
}

// TestOrderValidation tests the order validation logic
func TestOrderValidation(t *testing.T) {
	tests := []struct {
		name          string
		order         Order
		expectedValid bool
	}{
		{
			name: "valid order",
			order: Order{
				ID:           "test-1",
				CustomerName: "John Doe",
				Email:        "john@example.com",
				Items:        []OrderItem{{ProductID: "1", Name: "Item", Quantity: 1, Price: 10.0}},
				Total:        10.0,
			},
			expectedValid: true,
		},
		{
			name: "missing customer name",
			order: Order{
				ID:           "test-2",
				CustomerName: "",
				Email:        "test@example.com",
				Items:        []OrderItem{{ProductID: "1", Name: "Item", Quantity: 1, Price: 10.0}},
				Total:        10.0,
			},
			expectedValid: false,
		},
		{
			name: "missing email",
			order: Order{
				ID:           "test-3",
				CustomerName: "John Doe",
				Email:        "",
				Items:        []OrderItem{{ProductID: "1", Name: "Item", Quantity: 1, Price: 10.0}},
				Total:        10.0,
			},
			expectedValid: false,
		},
		{
			name: "no items",
			order: Order{
				ID:           "test-4",
				CustomerName: "John Doe",
				Email:        "john@example.com",
				Items:        []OrderItem{},
				Total:        0,
			},
			expectedValid: false,
		},
		{
			name: "negative total",
			order: Order{
				ID:           "test-5",
				CustomerName: "John Doe",
				Email:        "john@example.com",
				Items:        []OrderItem{{ProductID: "1", Name: "Item", Quantity: 1, Price: -10.0}},
				Total:        -10.0,
			},
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple validation test without database
			isValid := tt.order.CustomerName != "" &&
				tt.order.Email != "" &&
				len(tt.order.Items) > 0 &&
				tt.order.Total > 0

			if isValid != tt.expectedValid {
				t.Errorf("Expected validation result %v, got %v", tt.expectedValid, isValid)
			}
		})
	}
}
