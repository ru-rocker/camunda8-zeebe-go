package model

import "time"

// OrderStatus represents the current state of the order
type OrderStatus string

const (
	OrderStatusCreated   OrderStatus = "CREATED"
	OrderStatusValidated OrderStatus = "VALIDATED"
	OrderStatusPaid      OrderStatus = "PAID"
	OrderStatusShipped   OrderStatus = "SHIPPED"
	OrderStatusFailed    OrderStatus = "FAILED"
)

// OrderItem represents an item within an order
type OrderItem struct {
	ProductID string  `json:"productId"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
}

// OrderPayload represents the complete workflow variables passed in Camunda 8
type OrderPayload struct {
	OrderID                 string      `json:"orderId"`
	CustomerID              string      `json:"customerId"`
	CustomerTier            string      `json:"customerTier,omitempty"`
	FraudScore              float64     `json:"fraudScore,omitempty"`
	Items                   []OrderItem `json:"items"`
	TotalAmount             float64     `json:"totalAmount"`
	DiscountPercent         float64     `json:"discountPercent,omitempty"`
	DiscountedAmount        float64     `json:"discountedAmount,omitempty"`
	RiskLevel               string      `json:"riskLevel,omitempty"`
	RequiresManagerApproval bool        `json:"requiresManagerApproval,omitempty"`
	Status                  OrderStatus `json:"status"`
	RequiresManualReview    bool        `json:"requiresManualReview,omitempty"`
	OrderApproved           bool        `json:"orderApproved,omitempty"`
	ManagerApproved         bool        `json:"managerApproved,omitempty"`
	ReviewedBy              string      `json:"reviewedBy,omitempty"`
	InventoryReserved       bool        `json:"inventoryReserved,omitempty"`
	PaymentID               string      `json:"paymentId,omitempty"`
	TrackingNumber          string      `json:"trackingNumber,omitempty"`
	FailureReason           string      `json:"failureReason,omitempty"`
	CreatedAt               time.Time   `json:"createdAt"`
	UpdatedAt               time.Time   `json:"updatedAt"`
}

// PaymentRequest contains details required to process payment
type PaymentRequest struct {
	OrderID     string  `json:"orderId"`
	CustomerID  string  `json:"customerId"`
	TotalAmount float64 `json:"totalAmount"`
	CardToken   string  `json:"cardToken,omitempty"`
}

// PaymentResponse contains result from payment processing
type PaymentResponse struct {
	PaymentID string `json:"paymentId"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
}

// ShippingDetails contains tracking and delivery info
type ShippingDetails struct {
	TrackingNumber string    `json:"trackingNumber"`
	Carrier        string    `json:"carrier"`
	EstimatedDate  time.Time `json:"estimatedDate"`
}
