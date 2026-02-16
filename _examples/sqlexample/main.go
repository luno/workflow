package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/sqlstore"
)

// OrderStatus represents the states an order can be in
type OrderStatus int

const (
	OrderUnknown           OrderStatus = 0
	OrderCreated           OrderStatus = 1
	OrderValidated         OrderStatus = 2
	OrderPaymentProcessed  OrderStatus = 3
	OrderFulfilled         OrderStatus = 4
	OrderCompleted         OrderStatus = 5
	OrderRejected          OrderStatus = 6
	OrderPaymentFailed     OrderStatus = 7
	OrderFulfillmentFailed OrderStatus = 8
)

func (s OrderStatus) String() string {
	switch s {
	case OrderCreated:
		return "Created"
	case OrderValidated:
		return "Validated"
	case OrderPaymentProcessed:
		return "PaymentProcessed"
	case OrderFulfilled:
		return "Fulfilled"
	case OrderCompleted:
		return "Completed"
	case OrderRejected:
		return "Rejected"
	case OrderPaymentFailed:
		return "PaymentFailed"
	case OrderFulfillmentFailed:
		return "FulfillmentFailed"
	default:
		return "Unknown"
	}
}

// Order represents an e-commerce order
type Order struct {
	ID           string
	CustomerName string
	Email        string
	Items        []OrderItem
	Total        float64
	PaymentID    string
	TrackingID   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// OrderItem represents a line item in an order
type OrderItem struct {
	ProductID string
	Name      string
	Quantity  int
	Price     float64
}

func main() {
	fmt.Println("=== Workflow SQL Example ===")
	fmt.Println("Demonstrating MariaDB/MySQL integration with Workflow")
	fmt.Println()

	// 1. Connect to MariaDB
	dsn := "workflow_user:workflow_pass@tcp(localhost:3306)/workflow_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 2. Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// 3. Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Database ping failed: %v\n\nMake sure MariaDB is running:\n  docker-compose up -d\n\nError: %v", err, err)
	}

	fmt.Println("✅ Connected to MariaDB")

	// 4. Verify schema exists
	if err := verifySchema(db); err != nil {
		log.Fatalf("❌ Schema verification failed: %v", err)
	}
	fmt.Println("✅ Schema verified")
	fmt.Println()

	// 5. Create SQL store adapter (writer and reader can be the same DB)
	store := sqlstore.New(db, db, "workflow_records", "workflow_outbox")

	// 6. Build workflow with SQL persistence
	wf := buildOrderWorkflow(store)

	// 7. Run workflow engine
	ctx := context.Background()
	wf.Run(ctx)
	defer wf.Stop()

	fmt.Println("✅ Workflow engine started")
	fmt.Println()

	// 8. Process some orders
	orders := generateSampleOrders()

	fmt.Printf("Processing %d orders...\n\n", len(orders))

	for i, order := range orders {
		fmt.Printf("📦 Order %d/%d: %s (Customer: %s, Total: $%.2f)\n",
			i+1, len(orders), order.ID, order.CustomerName, order.Total)

		// Trigger workflow
		runID, err := wf.Trigger(ctx, order.ID, workflow.WithInitialValue[Order, OrderStatus](&order))
		if err != nil {
			fmt.Printf("   ❌ Failed to trigger workflow: %v\n\n", err)
			continue
		}

		fmt.Printf("   🚀 Workflow started (run_id: %s)\n", runID)

		// Wait for completion with timeout
		ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		run, err := wf.Await(ctxTimeout, order.ID, runID, OrderCompleted)
		cancel()

		if err != nil {
			fmt.Printf("   ⏱️  Workflow still processing or timed out\n\n")
			continue
		}

		// Display result based on final status
		switch run.Status {
		case OrderCompleted:
			fmt.Printf("   ✅ Order completed! (Tracking: %s)\n", run.Object.TrackingID)
		case OrderRejected:
			fmt.Printf("   ❌ Order rejected during validation\n")
		case OrderPaymentFailed:
			fmt.Printf("   💳 Payment failed\n")
		case OrderFulfillmentFailed:
			fmt.Printf("   📦 Fulfillment failed\n")
		default:
			fmt.Printf("   ℹ️  Order in status: %s\n", run.Status)
		}

		fmt.Println()

		// Small delay to see workflow progression
		time.Sleep(100 * time.Millisecond)
	}

	// 9. Display database statistics
	fmt.Println("\n=== Database Statistics ===")
	displayDatabaseStats(db)

	fmt.Println("\n=== Example Complete ===")
	fmt.Println("💡 Tip: Connect to MariaDB to inspect the data:")
	fmt.Println("   docker exec -it sqlexample-mariadb mysql -uworkflow_user -pworkflow_pass workflow_db")
	fmt.Println("\n   Then run queries like:")
	fmt.Println("   SELECT workflow_name, foreign_id, status, created_at FROM workflow_records ORDER BY created_at DESC;")
}

func buildOrderWorkflow(store workflow.RecordStore) *workflow.Workflow[Order, OrderStatus] {
	b := workflow.NewBuilder[Order, OrderStatus]("order-processor")

	// Step 1: Validate order
	b.AddStep(OrderCreated,
		func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
			// Validation logic
			if r.Object.CustomerName == "" {
				return OrderRejected, fmt.Errorf("customer name is required")
			}
			if r.Object.Email == "" {
				return OrderRejected, fmt.Errorf("email is required")
			}
			if len(r.Object.Items) == 0 {
				return OrderRejected, fmt.Errorf("order must have at least one item")
			}
			if r.Object.Total <= 0 {
				return OrderRejected, fmt.Errorf("order total must be positive")
			}

			r.Object.UpdatedAt = time.Now()
			return OrderValidated, nil
		},
		OrderValidated,
		OrderRejected,
	)

	// Step 2: Process payment
	b.AddStep(OrderValidated,
		func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
			// Simulate payment processing
			time.Sleep(50 * time.Millisecond)

			// Simulate occasional payment failures
			if rand.Float32() < 0.1 { // 10% failure rate
				return OrderPaymentFailed, fmt.Errorf("payment gateway declined")
			}

			// Generate payment ID
			r.Object.PaymentID = fmt.Sprintf("PAY-%d", time.Now().Unix())
			r.Object.UpdatedAt = time.Now()

			return OrderPaymentProcessed, nil
		},
		OrderPaymentProcessed,
		OrderPaymentFailed,
	)

	// Step 3: Fulfill order
	b.AddStep(OrderPaymentProcessed,
		func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
			// Simulate order fulfillment
			time.Sleep(50 * time.Millisecond)

			// Simulate occasional fulfillment failures
			if rand.Float32() < 0.05 { // 5% failure rate
				return OrderFulfillmentFailed, fmt.Errorf("item out of stock")
			}

			// Generate tracking ID
			r.Object.TrackingID = fmt.Sprintf("TRACK-%d", time.Now().Unix())
			r.Object.UpdatedAt = time.Now()

			return OrderFulfilled, nil
		},
		OrderFulfilled,
		OrderFulfillmentFailed,
	)

	// Step 4: Complete order
	b.AddStep(OrderFulfilled,
		func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
			r.Object.UpdatedAt = time.Now()
			return OrderCompleted, nil
		},
		OrderCompleted,
	)

	return b.Build(
		memstreamer.New(),
		store, // SQL record store
		memrolescheduler.New(),
	)
}

func verifySchema(db *sql.DB) error {
	tables := []string{"workflow_records", "workflow_outbox"}

	for _, table := range tables {
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*) 
			FROM information_schema.tables 
			WHERE table_schema = 'workflow_db' 
			AND table_name = ?
		`, table).Scan(&count)

		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}

		if count != 1 {
			return fmt.Errorf("table %s not found - run: docker-compose up -d", table)
		}
	}

	return nil
}

func displayDatabaseStats(db *sql.DB) {
	// Count workflow records by status
	rows, err := db.Query(`
		SELECT status, COUNT(*) as count 
		FROM workflow_records 
		WHERE workflow_name = 'order-processor'
		GROUP BY status
		ORDER BY status
	`)
	if err != nil {
		fmt.Printf("Failed to query stats: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\nWorkflow Records by Status:")
	statusCount := make(map[OrderStatus]int)
	for rows.Next() {
		var status int
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		statusCount[OrderStatus(status)] = count
	}

	for status, count := range statusCount {
		fmt.Printf("  %s: %d\n", status, count)
	}

	// Count outbox events
	var outboxCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM workflow_outbox 
		WHERE workflow_name = 'order-processor'
	`).Scan(&outboxCount)
	if err != nil {
		fmt.Printf("\nFailed to query outbox count: %v\n", err)
	} else {
		fmt.Printf("\nPending Outbox Events: %d\n", outboxCount)
	}

	// Total records
	var totalRecords int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM workflow_records 
		WHERE workflow_name = 'order-processor'
	`).Scan(&totalRecords)
	if err != nil {
		fmt.Printf("Failed to query total records: %v\n", err)
	} else {
		fmt.Printf("Total Records: %d\n", totalRecords)
	}
}

func generateSampleOrders() []Order {
	rand.Seed(time.Now().UnixNano())

	return []Order{
		{
			ID:           "order-1001",
			CustomerName: "Alice Johnson",
			Email:        "alice@example.com",
			Items: []OrderItem{
				{ProductID: "prod-1", Name: "Laptop", Quantity: 1, Price: 1299.99},
				{ProductID: "prod-2", Name: "Mouse", Quantity: 2, Price: 29.99},
			},
			Total:     1359.97,
			CreatedAt: time.Now(),
		},
		{
			ID:           "order-1002",
			CustomerName: "Bob Smith",
			Email:        "bob@example.com",
			Items: []OrderItem{
				{ProductID: "prod-3", Name: "Keyboard", Quantity: 1, Price: 149.99},
			},
			Total:     149.99,
			CreatedAt: time.Now(),
		},
		{
			ID:           "order-1003",
			CustomerName: "Carol Williams",
			Email:        "carol@example.com",
			Items: []OrderItem{
				{ProductID: "prod-4", Name: "Monitor", Quantity: 2, Price: 399.99},
				{ProductID: "prod-5", Name: "HDMI Cable", Quantity: 2, Price: 19.99},
			},
			Total:     839.96,
			CreatedAt: time.Now(),
		},
		{
			ID:           "order-1004",
			CustomerName: "",
			Email:        "invalid@example.com",
			Items: []OrderItem{
				{ProductID: "prod-6", Name: "Headphones", Quantity: 1, Price: 199.99},
			},
			Total:     199.99,
			CreatedAt: time.Now(),
		},
		{
			ID:           "order-1005",
			CustomerName: "David Brown",
			Email:        "david@example.com",
			Items: []OrderItem{
				{ProductID: "prod-7", Name: "Webcam", Quantity: 1, Price: 89.99},
				{ProductID: "prod-8", Name: "Microphone", Quantity: 1, Price: 129.99},
			},
			Total:     219.98,
			CreatedAt: time.Now(),
		},
	}
}
