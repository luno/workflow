# Callbacks

Callbacks allow workflows to receive and process external events, such as webhooks, manual interventions, or API calls. This enables workflows to respond to real-world events that happen outside the normal step progression.

## What are Callbacks?

A callback is a special type of workflow step that:
- Waits for external input at a specific status
- Can be triggered by external systems via HTTP endpoints, message queues, etc.
- Receives arbitrary data that can be processed to determine the next workflow state
- Enables human-in-the-loop and external system integration patterns

## Adding Callbacks

Use `AddCallback` to define a callback handler for a specific status:

```go
b := workflow.NewBuilder[Order, OrderStatus]("order-processing")

b.AddCallback(OrderStatusPendingApproval, func(
    ctx context.Context,
    r *workflow.Run[Order, OrderStatus],
    reader io.Reader,
) (OrderStatus, error) {
    // Read and parse the callback payload
    var approval ApprovalDecision
    if err := json.NewDecoder(reader).Decode(&approval); err != nil {
        return 0, fmt.Errorf("failed to parse approval: %w", err)
    }

    // Process the approval decision
    r.Object.ApprovalDecision = approval.Decision
    r.Object.ApprovedBy = approval.UserID
    r.Object.ApprovalNotes = approval.Notes
    r.Object.ApprovedAt = time.Now()

    // Transition based on decision
    if approval.Decision == "approved" {
        return OrderStatusApproved, nil
    }
    return OrderStatusRejected, nil
}, OrderStatusApproved, OrderStatusRejected)
```

## Triggering Callbacks

Use the workflow's `Callback` method to trigger a callback from external code:

```go
// In your HTTP handler, webhook endpoint, etc.
func handleApprovalWebhook(w http.ResponseWriter, r *http.Request) {
    orderID := r.URL.Query().Get("order_id")

    // Trigger the callback with the request body
    err := workflow.Callback(ctx, orderID, OrderStatusPendingApproval, r.Body)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}
```

## Common Callback Patterns

### 1. Approval Workflows

```go
type ApprovalDecision struct {
    Decision string    `json:"decision"` // "approved" or "rejected"
    UserID   string    `json:"user_id"`
    Notes    string    `json:"notes"`
    Timestamp time.Time `json:"timestamp"`
}

b.AddCallback(OrderStatusPendingManagerApproval, func(
    ctx context.Context,
    r *workflow.Run[Order, OrderStatus],
    reader io.Reader,
) (OrderStatus, error) {
    var decision ApprovalDecision
    if err := json.NewDecoder(reader).Decode(&decision); err != nil {
        return 0, err
    }

    // Validate approver has permission
    if !canApproveOrder(decision.UserID, r.Object) {
        return 0, fmt.Errorf("user %s not authorized to approve this order", decision.UserID)
    }

    r.Object.ManagerApproval = &decision

    switch decision.Decision {
    case "approved":
        return OrderStatusManagerApproved, nil
    case "rejected":
        return OrderStatusManagerRejected, nil
    default:
        return 0, fmt.Errorf("invalid decision: %s", decision.Decision)
    }
}, OrderStatusManagerApproved, OrderStatusManagerRejected)
```

### 2. Document Upload

```go
type DocumentUpload struct {
    DocumentType string `json:"document_type"`
    FileURL      string `json:"file_url"`
    UploadedBy   string `json:"uploaded_by"`
}

b.AddCallback(UserStatusAwaitingDocuments, func(
    ctx context.Context,
    r *workflow.Run[User, UserStatus],
    reader io.Reader,
) (UserStatus, error) {
    var upload DocumentUpload
    if err := json.NewDecoder(reader).Decode(&upload); err != nil {
        return 0, err
    }

    // Store document reference
    if r.Object.Documents == nil {
        r.Object.Documents = make(map[string]string)
    }
    r.Object.Documents[upload.DocumentType] = upload.FileURL

    // Check if all required documents are uploaded
    requiredDocs := []string{"identity", "proof_of_address", "bank_statement"}
    for _, docType := range requiredDocs {
        if _, exists := r.Object.Documents[docType]; !exists {
            return UserStatusAwaitingDocuments, nil // Still waiting for more documents
        }
    }

    return UserStatusDocumentsComplete, nil
}, UserStatusAwaitingDocuments, UserStatusDocumentsComplete)
```

### 3. Payment Confirmation

```go
type PaymentWebhook struct {
    PaymentID string `json:"payment_id"`
    Status    string `json:"status"`
    Amount    float64 `json:"amount"`
    Currency  string `json:"currency"`
}

b.AddCallback(OrderStatusPaymentPending, func(
    ctx context.Context,
    r *workflow.Run[Order, OrderStatus],
    reader io.Reader,
) (OrderStatus, error) {
    var webhook PaymentWebhook
    if err := json.NewDecoder(reader).Decode(&webhook); err != nil {
        return 0, err
    }

    // Verify payment belongs to this order
    if webhook.PaymentID != r.Object.PaymentID {
        return 0, fmt.Errorf("payment ID mismatch")
    }

    // Update order with payment status
    r.Object.PaymentStatus = webhook.Status
    r.Object.PaidAmount = webhook.Amount

    switch webhook.Status {
    case "completed":
        return OrderStatusPaymentCompleted, nil
    case "failed":
        return OrderStatusPaymentFailed, nil
    case "pending":
        return OrderStatusPaymentPending, nil // Stay in same state
    default:
        return 0, fmt.Errorf("unknown payment status: %s", webhook.Status)
    }
}, OrderStatusPaymentCompleted, OrderStatusPaymentFailed, OrderStatusPaymentPending)
```

### 4. External System Integration

```go
type ThirdPartyVerification struct {
    VerificationID string `json:"verification_id"`
    Status         string `json:"status"`
    Score          int    `json:"score"`
    Flags          []string `json:"flags"`
}

b.AddCallback(UserStatusPendingVerification, func(
    ctx context.Context,
    r *workflow.Run[User, UserStatus],
    reader io.Reader,
) (UserStatus, error) {
    var verification ThirdPartyVerification
    if err := json.NewDecoder(reader).Decode(&verification); err != nil {
        return 0, err
    }

    r.Object.VerificationScore = verification.Score
    r.Object.VerificationFlags = verification.Flags

    // Apply business logic
    if verification.Status == "verified" && verification.Score >= 80 {
        return UserStatusVerified, nil
    } else if verification.Score < 50 || len(verification.Flags) > 2 {
        return UserStatusVerificationFailed, nil
    } else {
        // Requires manual review
        return UserStatusPendingManualReview, nil
    }
}, UserStatusVerified, UserStatusVerificationFailed, UserStatusPendingManualReview)
```

## Error Handling

Callbacks support the same error handling patterns as regular steps:

```go
b.AddCallback(OrderStatusAwaitingCustomerInput, func(
    ctx context.Context,
    r *workflow.Run[Order, OrderStatus],
    reader io.Reader,
) (OrderStatus, error) {
    var input CustomerInput
    if err := json.NewDecoder(reader).Decode(&input); err != nil {
        // Invalid input - this will retry
        return 0, fmt.Errorf("failed to parse customer input: %w", err)
    }

    // Validate input
    if err := validateCustomerInput(input); err != nil {
        // Business validation failed - store error but don't retry
        r.Object.ValidationErrors = append(r.Object.ValidationErrors, err.Error())
        return OrderStatusCustomerInputInvalid, nil
    }

    // Process valid input
    r.Object.CustomerInput = input
    return OrderStatusCustomerInputReceived, nil
}, OrderStatusCustomerInputReceived, OrderStatusCustomerInputInvalid)
```

## Multiple Callbacks per Status

You can add multiple callbacks for the same status to handle different event types:

```go
// Handle email responses
b.AddCallback(UserStatusEmailSent, handleEmailResponse,
    UserStatusEmailConfirmed, UserStatusEmailBounced)

// Handle phone responses
b.AddCallback(UserStatusEmailSent, handlePhoneResponse,
    UserStatusPhoneConfirmed, UserStatusPhoneFailed)

// Handle web responses
b.AddCallback(UserStatusEmailSent, handleWebConfirmation,
    UserStatusWebConfirmed)
```

## Production Considerations

### Security

```go
b.AddCallback(OrderStatusPendingApproval, func(
    ctx context.Context,
    r *workflow.Run[Order, OrderStatus],
    reader io.Reader,
) (OrderStatus, error) {
    // Verify webhook signature
    signature := getSignatureFromContext(ctx)
    if !verifyWebhookSignature(reader, signature) {
        return 0, errors.New("invalid webhook signature")
    }

    // Rate limiting
    if isRateLimited(ctx) {
        return 0, errors.New("rate limited")
    }

    // Process callback...
    return OrderStatusApproved, nil
}, OrderStatusApproved, OrderStatusRejected)
```

### Idempotency

```go
type CallbackPayload struct {
    IdempotencyKey string `json:"idempotency_key"`
    // ... other fields
}

b.AddCallback(OrderStatusPendingPayment, func(
    ctx context.Context,
    r *workflow.Run[Order, OrderStatus],
    reader io.Reader,
) (OrderStatus, error) {
    var payload CallbackPayload
    if err := json.NewDecoder(reader).Decode(&payload); err != nil {
        return 0, err
    }

    // Check if already processed
    if r.Object.ProcessedCallbacks == nil {
        r.Object.ProcessedCallbacks = make(map[string]bool)
    }

    if r.Object.ProcessedCallbacks[payload.IdempotencyKey] {
        // Already processed - return current state without changing
        return OrderStatusPendingPayment, nil
    }

    // Mark as processed
    r.Object.ProcessedCallbacks[payload.IdempotencyKey] = true

    // Process the callback...
    return OrderStatusPaymentReceived, nil
}, OrderStatusPaymentReceived)
```

### Timeouts with Callbacks

Combine callbacks with timeouts for robust handling:

```go
// Add timeout for approval
b.AddTimeout(
    OrderStatusPendingApproval,
    workflow.DurationTimerFunc[Order, OrderStatus](24*time.Hour),
    func(ctx context.Context, r *workflow.Run[Order, OrderStatus], now time.Time) (OrderStatus, error) {
        // Auto-reject after 24 hours
        r.Object.RejectionReason = "approval timeout"
        return OrderStatusAutoRejected, nil
    },
    OrderStatusAutoRejected,
)

// Add callback for manual approval
b.AddCallback(OrderStatusPendingApproval, handleApproval,
    OrderStatusManuallyApproved, OrderStatusManuallyRejected)
```

## Testing Callbacks

```go
func TestApprovalCallback(t *testing.T) {
    wf := NewOrderWorkflow()
    defer wf.Stop()

    ctx := context.Background()
    wf.Run(ctx)

    // Create order and advance to pending approval
    order := &Order{ID: "order-123", Total: 500.00}
    runID, err := wf.Trigger(ctx, order.ID, workflow.WithInitialValue(order))
    require.NoError(t, err)

    // Wait for pending approval state
    run, err := wf.Await(ctx, order.ID, runID, OrderStatusPendingApproval)
    require.NoError(t, err)

    // Test approval callback
    approvalPayload := ApprovalDecision{
        Decision: "approved",
        UserID:   "manager-123",
        Notes:    "Order looks good",
    }

    payloadBytes, _ := json.Marshal(approvalPayload)
    err = wf.Callback(ctx, order.ID, OrderStatusPendingApproval, bytes.NewReader(payloadBytes))
    require.NoError(t, err)

    // Verify approval was processed
    finalRun, err := wf.Await(ctx, order.ID, runID, OrderStatusApproved)
    require.NoError(t, err)
    assert.Equal(t, "approved", finalRun.Object.ApprovalDecision)
    assert.Equal(t, "manager-123", finalRun.Object.ApprovedBy)
}
```

Callbacks are powerful for integrating workflows with external systems and enabling human-in-the-loop processes. Use them whenever your workflow needs to wait for and respond to external events.

## Next Steps

- **[Timeouts](timeouts.md)** - Add time-based operations and deadlines
- **[Connectors](connectors.md)** - Connect to external event streams
- **[Hooks](hooks.md)** - React to workflow lifecycle changes
- **[Examples](examples/)** - See callback patterns in real workflows