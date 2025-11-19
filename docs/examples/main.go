package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/luno/workflow"
)

func main() {
	fmt.Println("🚀 Starting Order Processing Workflow Example")
	fmt.Println(strings.Repeat("=", 60))

	wf := NewOrderWorkflow()

	ctx := context.Background()
	wf.Run(ctx)
	defer wf.Stop()

	// Create sample orders
	orders := []*Order{
		{
			ID:            "order-001",
			CustomerID:    "customer-123",
			Total:         99.99,
			PaymentMethod: "credit_card",
			Items: []OrderItem{
				{ProductID: "laptop", Quantity: 1, Price: 99.99},
			},
			ShippingAddress: Address{
				Street:  "123 Main St",
				City:    "San Francisco",
				State:   "CA",
				ZipCode: "94105",
				Country: "USA",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:            "order-002",
			CustomerID:    "premium_customer",
			Total:         1500.00,
			PaymentMethod: "credit_card", // Will fail and retry with alternative
			Items: []OrderItem{
				{ProductID: "desktop", Quantity: 1, Price: 1500.00},
			},
			ShippingAddress: Address{
				Street:  "456 Tech Ave",
				City:    "Austin",
				State:   "TX",
				ZipCode: "78701",
				Country: "USA",
			},
			CreatedAt: time.Now(),
		},
		{
			ID:            "order-003",
			CustomerID:    "customer-456",
			Total:         49.99,
			PaymentMethod: "credit_card",
			Items: []OrderItem{
				{ProductID: "out_of_stock", Quantity: 1, Price: 49.99},
			},
			ShippingAddress: Address{
				Street:  "789 Commerce Blvd",
				City:    "Seattle",
				State:   "WA",
				ZipCode: "98101",
				Country: "USA",
			},
			CreatedAt: time.Now(),
		},
	}

	var wg sync.WaitGroup
	wg.Add(len(orders))
	// Process orders
	for _, order := range orders {
		fmt.Printf("🚀 [%s] Triggering order workflow\n", order.ID)

		runID, err := wf.Trigger(ctx, order.ID, workflow.WithInitialValue[Order, OrderStatus](order))
		if err != nil {
			panic(err)
		}

		// Monitor progress (in real app, this would be done asynchronously)
		go func(orderID string) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			run, err := wf.Await(ctx, orderID, runID, OrderStatusFulfilled)
			if err != nil {
				panic(err)
			}

			wg.Done()
			fmt.Printf("✅ [%s] Workflow completed with status: %s\n", orderID, run.Status)
		}(order.ID)
	}

	// Let workflows complete
	fmt.Println("\n⏳ Waiting for workflows to complete...")
	wg.Wait()
	fmt.Println("\n🎉 Order Processing Example completed!")
}
