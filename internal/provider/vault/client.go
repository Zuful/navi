package vault

import (
	"context"
	"time"
)

// BillingClient abstracts billing backend operations.
type BillingClient interface {
	GetSubscriptionStatus(ctx context.Context, customerID string) (*Subscription, error)
	GetRevenueMetrics(ctx context.Context, customerID string, months int) (*RevenueMetrics, error)
	GetRenewalCalendar(ctx context.Context, daysAhead int) ([]Renewal, error)
}

// Subscription represents a customer's subscription details.
type Subscription struct {
	CustomerID   string
	CustomerName string
	Plan         string
	Status       string // active, trialing, past_due, canceled, unpaid
	MRR          int64  // in cents
	Currency     string
	TrialEnd     *time.Time
	CurrentPeriodEnd time.Time
	CancelAt     *time.Time
	CreatedAt    time.Time
}

// RevenueMetrics contains billing metrics for a customer.
type RevenueMetrics struct {
	CustomerID         string
	CustomerName       string
	CurrentMRR         int64
	Currency           string
	MRRHistory         []MRRDataPoint
	PaymentSuccessRate float64
	OutstandingInvoices []Invoice
}

// MRRDataPoint is a single month's MRR value.
type MRRDataPoint struct {
	Month string // "2024-01"
	MRR   int64  // in cents
}

// Invoice represents an outstanding invoice.
type Invoice struct {
	ID        string
	Amount    int64 // in cents
	Currency  string
	Status    string
	DueDate   *time.Time
	CreatedAt time.Time
}

// Renewal represents an upcoming subscription renewal or expiration.
type Renewal struct {
	CustomerID   string
	CustomerName string
	Plan         string
	MRR          int64
	Currency     string
	RenewalDate  time.Time
	Status       string // renewing, expiring, trial_ending
}
