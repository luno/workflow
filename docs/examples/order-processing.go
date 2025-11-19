package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
)

// OrderStatus represents all possible states in our order workflow
type OrderStatus int

const (
	OrderStatusUnknown           OrderStatus = 0
	OrderStatusCreated           OrderStatus = 1
	OrderStatusValidated         OrderStatus = 2
	OrderStatusPaymentProcessing OrderStatus = 3
	OrderStatusPaymentProcessed  OrderStatus = 4
	OrderStatusPaymentFailed     OrderStatus = 5
	OrderStatusInventoryReserved OrderStatus = 6
	OrderStatusInventoryFailed   OrderStatus = 7
	OrderStatusFulfilled         OrderStatus = 8
	OrderStatusCancelled         OrderStatus = 9
	OrderStatusRefunded          OrderStatus = 10
)

func (s OrderStatus) String() string {
	switch s {
	case OrderStatusCreated:
		return "Created"
	case OrderStatusValidated:
		return "Validated"
	case OrderStatusPaymentProcessing:
		return "PaymentProcessing"
	case OrderStatusPaymentProcessed:
		return "PaymentProcessed"
	case OrderStatusPaymentFailed:
		return "PaymentFailed"
	case OrderStatusInventoryReserved:
		return "InventoryReserved"
	case OrderStatusInventoryFailed:
		return "InventoryFailed"
	case OrderStatusFulfilled:
		return "Fulfilled"
	case OrderStatusCancelled:
		return "Cancelled"
	case OrderStatusRefunded:
		return "Refunded"
	default:
		return "Unknown"
	}
}

// Order represents our business entity flowing through the workflow
type Order struct {
	ID              string      `json:"id"`
	CustomerID      string      `json:"customer_id"`
	Items           []OrderItem `json:"items"`
	Total           float64     `json:"total"`
	PaymentMethod   string      `json:"payment_method"`
	ShippingAddress Address     `json:"shipping_address"`

	// Workflow state
	ValidationErrors []string `json:"validation_errors,omitempty"`
	PaymentID        string   `json:"payment_id,omitempty"`
	ReservationID    string   `json:"reservation_id,omitempty"`
	TrackingNumber   string   `json:"tracking_number,omitempty"`
	PaymentAttempts  int      `json:"payment_attempts"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	ValidatedAt *time.Time `json:"validated_at,omitempty"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	FulfilledAt *time.Time `json:"fulfilled_at,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
}

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

// Mock external services
var (
	inventoryService = &MockInventoryService{}
	paymentService   = &MockPaymentService{}
	shippingService  = &MockShippingService{}
)

func NewOrderWorkflow() *workflow.Workflow[Order, OrderStatus] {
	b := workflow.NewBuilder[Order, OrderStatus]("order-processing")

	// Step 1: Validate order
	b.AddStep(OrderStatusCreated, validateOrder, OrderStatusValidated, OrderStatusCancelled)

	// Step 2: Process payment (with retries)
	b.AddStep(OrderStatusValidated, processPayment, OrderStatusPaymentProcessing, OrderStatusPaymentFailed, OrderStatusCancelled)

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
		fmt.Printf("📊 [%s] Order completed in %v\n", record.Object.ID, time.Since(record.CreatedAt))
		return nil
	})

	b.OnPause(func(ctx context.Context, record *workflow.TypedRecord[Order, OrderStatus]) error {
		fmt.Printf("⚠️ [%s] Order paused due to errors - requires investigation\n", record.Object.ID)
		// In real implementation, send alert to operations team
		return nil
	})

	return b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
		workflow.WithTimeoutStore(memtimeoutstore.New()),
		workflow.WithDefaultOptions(
			workflow.ErrBackOff(time.Millisecond),
		),
	)
}

// Step implementations

func validateOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	fmt.Printf("🔍 [%s] Validating order\n", r.Object.ID)

	var validationErrors []string

	// Validate items
	if len(r.Object.Items) == 0 {
		validationErrors = append(validationErrors, "Order must have at least one item")
	}

	// Validate total
	calculatedTotal := 0.0
	for _, item := range r.Object.Items {
		if item.Quantity <= 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("Invalid quantity for product %s", item.ProductID))
		}
		if item.Price < 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("Invalid price for product %s", item.ProductID))
		}
		calculatedTotal += item.Price * float64(item.Quantity)
	}

	if abs(r.Object.Total-calculatedTotal) > 0.01 {
		validationErrors = append(validationErrors, "Order total does not match item prices")
	}

	// Validate payment method
	if r.Object.PaymentMethod == "" {
		validationErrors = append(validationErrors, "Payment method is required")
	}

	// Validate shipping address
	if r.Object.ShippingAddress.Street == "" || r.Object.ShippingAddress.City == "" {
		validationErrors = append(validationErrors, "Complete shipping address is required")
	}

	if len(validationErrors) > 0 {
		r.Object.ValidationErrors = validationErrors
		return OrderStatusCancelled, nil
	}

	now := time.Now()
	r.Object.ValidatedAt = &now
	fmt.Printf("✅ [%s] Order validated successfully\n", r.Object.ID)

	return OrderStatusValidated, nil
}

func processPayment(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	fmt.Printf("💳 [%s] Processing payment (attempt %d)\n", r.Object.ID, r.Object.PaymentAttempts+1)

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
			// Try again until no longer tryable or max attempts reached
			return r.SaveAndRepeat()
		}

		fmt.Printf("❌ [%s] Payment processing failed: %v\n", r.Object.ID, err)
		return OrderStatusPaymentFailed, nil
	}

	r.Object.PaymentID = paymentID
	return OrderStatusPaymentProcessing, nil
}

func completePayment(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	fmt.Printf("✅ [%s] Confirming payment %s\n", r.Object.ID, r.Object.PaymentID)

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
	fmt.Printf("📦 [%s] Reserving inventory\n", r.Object.ID)

	reservationID, err := inventoryService.ReserveItems(r.Object.Items)
	if err != nil {
		if errors.Is(err, ErrInsufficientInventory) {
			return OrderStatusInventoryFailed, nil
		}
		return 0, err // Retryable error
	}

	r.Object.ReservationID = reservationID
	fmt.Printf("✅ [%s] Inventory reserved with ID %s\n", r.Object.ID, reservationID)

	return OrderStatusInventoryReserved, nil
}

func fulfillOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	fmt.Printf("🚚 [%s] Fulfilling order\n", r.Object.ID)

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

	fmt.Printf("🎉 [%s] Order fulfilled! Tracking number: %s\n", r.Object.ID, trackingNumber)

	return OrderStatusFulfilled, nil
}

// Error handlers

func handlePaymentFailure(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	fmt.Printf("❌ [%s] Handling payment failure\n", r.Object.ID)

	// If we haven't exceeded retry attempts, try alternative payment method
	if r.Object.PaymentAttempts < 3 && hasAlternativePaymentMethod(r.Object.CustomerID) {
		fmt.Printf("🔄 [%s] Retrying payment with alternative method\n", r.Object.ID)
		r.Object.PaymentMethod = getAlternativePaymentMethod(r.Object.CustomerID)
		return OrderStatusPaymentProcessing, nil
	}

	return OrderStatusCancelled, nil
}

func handleInventoryFailure(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	fmt.Printf("📦 [%s] Handling inventory failure\n", r.Object.ID)

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
	fmt.Printf("🚫 [%s] Processing cancellation\n", r.Object.ID)

	if r.Object.PaymentID == "" && r.Object.ReservationID == "" {
		return r.Cancel(ctx, "No payment to refund")
	}

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

// Mock service implementations

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
		return "", errors.New("payment declined by bank")
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
