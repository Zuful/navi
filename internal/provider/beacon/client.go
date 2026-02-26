package beacon

import (
	"context"
	"time"
)

// SupportClient abstracts support ticket backend operations.
type SupportClient interface {
	GetOpenTickets(ctx context.Context, customerID string) ([]Ticket, error)
	GetTicketHistory(ctx context.Context, customerID string, limit int) ([]Ticket, error)
	GetSatisfactionScores(ctx context.Context, customerID string) (*SatisfactionMetrics, error)
}

// Ticket represents a support ticket.
type Ticket struct {
	ID             string
	CustomerID     string
	Subject        string
	Description    string
	Status         string // open, pending, hold, solved, closed
	Priority       string // low, normal, high, urgent
	Channel        string // email, chat, phone, web, api
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ResolvedAt     *time.Time
	AssignedTo     string
	RequesterName  string
	RequesterEmail string
}

// SatisfactionMetrics holds CSAT and resolution metrics for a customer.
type SatisfactionMetrics struct {
	CustomerID           string
	AverageCSAT          float64
	TotalRatings         int
	OpenCount            int
	PendingCount         int
	AverageResolutionHours float64
	FirstResponseHours   float64
}
