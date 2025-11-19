# Order Processing Workflow

A complete example showing how to build an e-commerce order processing workflow with payment handling, inventory management, and fulfillment using Workflow.

## Overview

This example demonstrates a real-world order processing system that:

- Validates orders and checks inventory
- Processes payments with retry logic
- Reserves inventory and fulfills orders
- Handles failures and cancellations gracefully
- Includes comprehensive error handling and monitoring

## Complete Implementation

```go
package main

import (
    "context"
    "fmt"
    "time"
    "errors"

    "github.com/luno/workflow"
    "github.com/luno/workflow/adapters/memstreamer"
    "github.com/luno/workflow/adapters/memrecordstore"
    "github.com/luno/workflow/adapters/memrolescheduler"
)

// OrderStatus represents all possible states in our order workflow
type OrderStatus int

const (
    OrderStatusUnknown         OrderStatus = 0
    OrderStatusCreated         OrderStatus = 1
    OrderStatusValidated       OrderStatus = 2
    OrderStatusPaymentProcessing OrderStatus = 3
    OrderStatusPaymentProcessed OrderStatus = 4
    OrderStatusPaymentFailed   OrderStatus = 5
    OrderStatusInventoryReserved OrderStatus = 6
    OrderStatusInventoryFailed OrderStatus = 7
    OrderStatusFulfilled       OrderStatus = 8
    OrderStatusCancelled       OrderStatus = 9
    OrderStatusRefunded        OrderStatus = 10
)

func (s OrderStatus) String() string {
    switch s {
    case OrderStatusCreated:         return "Created"
    case OrderStatusValidated:       return "Validated"
    case OrderStatusPaymentProcessing: return "PaymentProcessing"
    case OrderStatusPaymentProcessed: return "PaymentProcessed"
    case OrderStatusPaymentFailed:   return "PaymentFailed"
    case OrderStatusInventoryReserved: return "InventoryReserved"
    case OrderStatusInventoryFailed: return "InventoryFailed"
    case OrderStatusFulfilled:       return "Fulfilled"
    case OrderStatusCancelled:       return "Cancelled"
    case OrderStatusRefunded:        return "Refunded"
    default:                        return "Unknown"
    }
}

// Order represents our business entity flowing through the workflow
type Order struct {
    ID               string              `json:"id"`
    CustomerID       string              `json:"customer_id"`
    Items            []OrderItem         `json:"items"`
    Total            float64             `json:"total"`
    PaymentMethod    string              `json:"payment_method"`
    ShippingAddress  Address             `json:"shipping_address"`

    // Workflow state
    ValidationErrors []string            `json:"validation_errors,omitempty"`
    PaymentID        string              `json:"payment_id,omitempty"`
    ReservationID    string              `json:"reservation_id,omitempty"`
    TrackingNumber   string              `json:"tracking_number,omitempty"`
    PaymentAttempts  int                 `json:"payment_attempts"`

    // Timestamps
    CreatedAt        time.Time           `json:"created_at"`
    ValidatedAt      *time.Time          `json:"validated_at,omitempty"`
    PaidAt           *time.Time          `json:"paid_at,omitempty"`
    FulfilledAt      *time.Time          `json:"fulfilled_at,omitempty"`
    CancelledAt      *time.Time          `json:"cancelled_at,omitempty"`
}

type OrderItem struct {
    ProductID string  `json:"product_id"`
    Quantity  int     `json:"quantity"`
    Price     float64 `json:"price"`
}

type Address struct {
    Street   string `json:"street"`
    City     string `json:"city"`
    State    string `json:"state"`
    ZipCode  string `json:"zip_code"`
    Country  string `json:"country"`
}

// Mock external services (in real implementation, these would be actual service calls)
var (
    inventoryService = &MockInventoryService{}
    paymentService   = &MockPaymentService{}
    shippingService  = &MockShippingService{}
)

func NewOrderWorkflow() *workflow.Workflow[Order, OrderStatus] {
    b := workflow.NewBuilder[Order, OrderStatus]("order-processing")

    // Step 1: Validate order
    b.AddStep(OrderStatusCreated, validateOrder, OrderStatusValidated, OrderStatusCancelled).
        WithOptions(
            workflow.ErrBackOff(time.Second * 5),
            workflow.PauseAfterErrCount(3),
        )

    // Step 2: Process payment (with retries)
    b.AddStep(OrderStatusValidated, processPayment, OrderStatusPaymentProcessing, OrderStatusPaymentFailed, OrderStatusCancelled).
        WithOptions(
            workflow.ErrBackOff(time.Second * 10),
            workflow.PauseAfterErrCount(5),
        )

    // Step 3: Complete payment processing
    b.AddStep(OrderStatusPaymentProcessing, completePayment, OrderStatusPaymentProcessed, OrderStatusPaymentFailed)

    // Step 4: Reserve inventory
    b.AddStep(OrderStatusPaymentProcessed, reserveInventory, OrderStatusInventoryReserved, OrderStatusInventoryFailed)

    // Step 5: Fulfill order
    b.AddStep(OrderStatusInventoryReserved, fulfillOrder, OrderStatusFulfilled)

    // Error handling flows
    b.AddStep(OrderStatusPaymentFailed, handlePaymentFailure, OrderStatusCancelled, OrderStatusPaymentProcessing) // Allow retry
    b.AddStep(OrderStatusInventoryFailed, handleInventoryFailure, OrderStatusCancelled, OrderStatusRefunded)

    // Cancellation flow
    b.AddStep(OrderStatusCancelled, processCancellation, OrderStatusRefunded)

    // Add timeouts
    b.AddTimeout(
        OrderStatusPaymentProcessing,
        workflow.DurationTimerFunc[Order, OrderStatus](5*time.Minute),
        func(ctx context.Context, r *workflow.Run[Order, OrderStatus], now time.Time) (OrderStatus, error) {
            return OrderStatusPaymentFailed, nil
        },
        OrderStatusPaymentFailed,
    )

    // Add hooks for monitoring
    b.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, OrderStatus]) error {
        fmt.Printf("📊 Order %s completed in %v\n", record.Object.ID, time.Since(record.CreatedAt))
        return nil
    })

    b.OnPause(func(ctx context.Context, record *workflow.TypedRecord[Order, OrderStatus]) error {
        fmt.Printf("⚠️ Order %s paused due to errors - requires investigation\n", record.Object.ID)
        // In real implementation, send alert to operations team
        return nil
    })

    return b.Build(
        memstreamer.New(),
        memrecordstore.New(),
        memrolescheduler.New(),
    )
}

// Step implementations

func validateOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    fmt.Printf("🔍 Validating order %s\n", r.Object.ID)

    var errors []string

    // Validate items
    if len(r.Object.Items) == 0 {
        errors = append(errors, "Order must have at least one item")
    }

    // Validate total
    calculatedTotal := 0.0
    for _, item := range r.Object.Items {
        if item.Quantity <= 0 {
            errors = append(errors, fmt.Sprintf("Invalid quantity for product %s", item.ProductID))
        }
        if item.Price < 0 {
            errors = append(errors, fmt.Sprintf("Invalid price for product %s", item.ProductID))
        }
        calculatedTotal += item.Price * float64(item.Quantity)
    }

    if abs(r.Object.Total-calculatedTotal) > 0.01 {
        errors = append(errors, "Order total does not match item prices")
    }

    // Validate payment method
    if r.Object.PaymentMethod == "" {
        errors = append(errors, "Payment method is required")
    }

    // Validate shipping address
    if r.Object.ShippingAddress.Street == "" || r.Object.ShippingAddress.City == "" {
        errors = append(errors, "Complete shipping address is required")
    }

    if len(errors) > 0 {
        r.Object.ValidationErrors = errors
        return OrderStatusCancelled, nil
    }

    now := time.Now()
    r.Object.ValidatedAt = &now
    fmt.Printf("✅ Order %s validated successfully\n", r.Object.ID)

    return OrderStatusValidated, nil
}

func processPayment(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    fmt.Printf("💳 Processing payment for order %s (attempt %d)\n", r.Object.ID, r.Object.PaymentAttempts+1)

    r.Object.PaymentAttempts++

    // Simulate payment processing
    paymentID, err := paymentService.ProcessPayment(PaymentRequest{
        Amount:        r.Object.Total,
        PaymentMethod: r.Object.PaymentMethod,
        CustomerID:    r.Object.CustomerID,
        OrderID:       r.Object.ID,
    })

    if err != nil {
        if isRetryablePaymentError(err) && r.Object.PaymentAttempts < 3 {
            return 0, fmt.Errorf("payment failed (retryable): %w", err)
        }
        return OrderStatusPaymentFailed, nil
    }

    r.Object.PaymentID = paymentID
    return OrderStatusPaymentProcessing, nil
}

func completePayment(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    fmt.Printf("✅ Confirming payment %s for order %s\n", r.Object.PaymentID, r.Object.ID)

    // Check payment status
    status, err := paymentService.GetPaymentStatus(r.Object.PaymentID)
    if err != nil {
        return 0, err
    }

    switch status {
    case "completed":
        now := time.Now()
        r.Object.PaidAt = &now
        return OrderStatusPaymentProcessed, nil
    case "failed":
        return OrderStatusPaymentFailed, nil
    case "processing":
        // Still processing, check again later
        return OrderStatusPaymentProcessing, nil
    default:
        return OrderStatusPaymentFailed, nil
    }
}

func reserveInventory(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    fmt.Printf("📦 Reserving inventory for order %s\n", r.Object.ID)

    reservationID, err := inventoryService.ReserveItems(r.Object.Items)
    if err != nil {
        if errors.Is(err, ErrInsufficientInventory) {
            return OrderStatusInventoryFailed, nil
        }
        return 0, err // Retryable error
    }

    r.Object.ReservationID = reservationID
    fmt.Printf("✅ Inventory reserved with ID %s\n", reservationID)

    return OrderStatusInventoryReserved, nil
}

func fulfillOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    fmt.Printf("🚚 Fulfilling order %s\n", r.Object.ID)

    // Create shipment
    trackingNumber, err := shippingService.CreateShipment(ShipmentRequest{
        OrderID:         r.Object.ID,
        Items:           r.Object.Items,
        ShippingAddress: r.Object.ShippingAddress,
        ReservationID:   r.Object.ReservationID,
    })

    if err != nil {
        return 0, err
    }

    r.Object.TrackingNumber = trackingNumber
    now := time.Now()
    r.Object.FulfilledAt = &now

    fmt.Printf("🎉 Order %s fulfilled! Tracking number: %s\n", r.Object.ID, trackingNumber)

    return OrderStatusFulfilled, nil
}

// Error handlers

func handlePaymentFailure(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    fmt.Printf("❌ Handling payment failure for order %s\n", r.Object.ID)

    // If we haven't exceeded retry attempts, try alternative payment method
    if r.Object.PaymentAttempts < 3 && hasAlternativePaymentMethod(r.Object.CustomerID) {
        fmt.Printf("🔄 Retrying payment with alternative method\n")
        r.Object.PaymentMethod = getAlternativePaymentMethod(r.Object.CustomerID)
        return OrderStatusPaymentProcessing, nil
    }

    return OrderStatusCancelled, nil
}

func handleInventoryFailure(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    fmt.Printf("📦 Handling inventory failure for order %s\n", r.Object.ID)

    // Check if we have partial inventory
    availableItems, err := inventoryService.CheckPartialAvailability(r.Object.Items)
    if err != nil {
        return 0, err
    }

    if len(availableItems) > 0 && len(availableItems) < len(r.Object.Items) {
        // Offer partial fulfillment (simplified - in real app, would need customer consent)
        r.Object.Items = availableItems
        return OrderStatusInventoryReserved, nil
    }

    // No inventory available - refund payment
    return OrderStatusRefunded, nil
}

func processCancellation(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    fmt.Printf("🚫 Processing cancellation for order %s\n", r.Object.ID)

    // Release inventory reservation if exists
    if r.Object.ReservationID != "" {
        if err := inventoryService.ReleaseReservation(r.Object.ReservationID); err != nil {
            return 0, err
        }
    }

    // Refund payment if exists
    if r.Object.PaymentID != "" {
        if err := paymentService.RefundPayment(r.Object.PaymentID); err != nil {
            return 0, err
        }
    }

    now := time.Now()
    r.Object.CancelledAt = &now

    return OrderStatusRefunded, nil
}

// Mock service implementations (replace with real implementations)

type MockPaymentService struct{}

type PaymentRequest struct {
    Amount        float64
    PaymentMethod string
    CustomerID    string
    OrderID       string
}

func (s *MockPaymentService) ProcessPayment(req PaymentRequest) (string, error) {
    // Simulate payment processing
    if req.PaymentMethod == "invalid_card" {
        return "", errors.New("invalid payment method")
    }
    if req.Amount > 1000 && req.PaymentMethod == "credit_card" {
        return "", errors.New("payment declined by bank") // Retryable
    }

    return fmt.Sprintf("pay_%s_%d", req.OrderID, time.Now().Unix()), nil
}

func (s *MockPaymentService) GetPaymentStatus(paymentID string) (string, error) {
    // Simulate async payment processing
    return "completed", nil
}

func (s *MockPaymentService) RefundPayment(paymentID string) error {
    fmt.Printf("💰 Refunding payment %s\n", paymentID)
    return nil
}

type MockInventoryService struct{}

var ErrInsufficientInventory = errors.New("insufficient inventory")

func (s *MockInventoryService) ReserveItems(items []OrderItem) (string, error) {
    // Simulate inventory check
    for _, item := range items {
        if item.ProductID == "out_of_stock" {
            return "", ErrInsufficientInventory
        }
    }

    return fmt.Sprintf("res_%d", time.Now().Unix()), nil
}

func (s *MockInventoryService) CheckPartialAvailability(items []OrderItem) ([]OrderItem, error) {
    var available []OrderItem
    for _, item := range items {
        if item.ProductID != "out_of_stock" {
            available = append(available, item)
        }
    }
    return available, nil
}

func (s *MockInventoryService) ReleaseReservation(reservationID string) error {
    fmt.Printf("📦 Releasing reservation %s\n", reservationID)
    return nil
}

type MockShippingService struct{}

type ShipmentRequest struct {
    OrderID         string
    Items           []OrderItem
    ShippingAddress Address
    ReservationID   string
}

func (s *MockShippingService) CreateShipment(req ShipmentRequest) (string, error) {
    return fmt.Sprintf("TRACK_%s", req.OrderID), nil
}

// Helper functions

func isRetryablePaymentError(err error) bool {
    return err.Error() == "payment declined by bank" ||
           err.Error() == "network timeout"
}

func hasAlternativePaymentMethod(customerID string) bool {
    return customerID == "premium_customer"
}

func getAlternativePaymentMethod(customerID string) string {
    return "debit_card"
}

func abs(x float64) float64 {
    if x < 0 {
        return -x
    }
    return x
}

// Example usage

func main() {
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

    // Process orders
    for _, order := range orders {
        fmt.Printf("\n🚀 Starting order workflow for %s\n", order.ID)

        runID, err := wf.Trigger(ctx, order.ID, workflow.WithInitialValue(order))
        if err != nil {
            fmt.Printf("❌ Failed to start workflow: %v\n", err)
            continue
        }

        // Monitor progress (in real app, this would be done asynchronously)
        go func(orderID string) {
            ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
            defer cancel()

            run, err := wf.Await(ctx, orderID, runID, OrderStatusFulfilled, OrderStatusCancelled, OrderStatusRefunded)
            if err != nil {
                fmt.Printf("⏰ Order %s workflow timed out or failed: %v\n", orderID, err)
                return
            }

            fmt.Printf("✅ Order %s workflow completed with status: %s\n", orderID, run.Status)
        }(order.ID)
    }

    // Let workflows complete
    time.Sleep(10 * time.Second)
    fmt.Println("\n🎉 All order workflows completed!")
}
```

## Key Patterns Demonstrated

### 1. **Complex State Machine**
The workflow handles multiple paths including success, failure, and retry scenarios.

### 2. **Error Recovery**
Failed payments can retry with alternative methods, and inventory failures can offer partial fulfillment.

### 3. **Timeouts**
Payment processing has a timeout to prevent hanging indefinitely.

### 4. **External Service Integration**
Shows how to integrate with payment, inventory, and shipping services.

### 5. **Data Evolution**
The Order object accumulates data (payment IDs, tracking numbers) as it flows through steps.

### 6. **Monitoring & Alerting**
Uses hooks to track completion times and alert on paused workflows.

## Production Considerations

### Error Handling
- Payment failures distinguish between retryable and permanent errors
- Inventory failures attempt partial fulfillment before cancellation
- All external service calls include proper error handling

### Idempotency
- Use order ID and workflow run ID as idempotency keys for external services
- Ensure payment and inventory operations can be safely retried

### Monitoring
- Track order processing times and alert on anomalies
- Monitor payment failure rates and inventory availability
- Set up alerts for paused workflows requiring manual intervention

### Scaling
- Enable parallel processing for high-volume periods:
  ```go
  b.AddStep(OrderStatusCreated, validateOrder, OrderStatusValidated, OrderStatusCancelled).
      WithOptions(workflow.ParallelCount(10))
  ```

### Data Retention
- Implement archival for completed orders after business retention period
- Consider GDPR requirements for customer data deletion

This example provides a solid foundation for building production e-commerce workflows while demonstrating Workflow's key capabilities.