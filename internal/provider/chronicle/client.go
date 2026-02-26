package chronicle

import (
	"context"
	"time"
)

// CommsClient abstracts communications backend operations.
type CommsClient interface {
	GetRecentCommunications(ctx context.Context, customerID string, limit int) ([]Communication, error)
	GetContactTimeline(ctx context.Context, contactID string, limit int) ([]TimelineEvent, error)
}

// Communication represents a single communication (email, note, call, etc.).
type Communication struct {
	ID        string
	Type      string // email, note, call, meeting, task
	Subject   string
	Body      string
	Direction string // inbound, outbound
	Status    string
	CreatedAt time.Time
	ContactName string
	ContactEmail string
}

// TimelineEvent represents a chronological event in a contact's history.
type TimelineEvent struct {
	ID          string
	Type        string // email, note, call, meeting, deal_created, deal_closed
	Title       string
	Description string
	CreatedAt   time.Time
	Metadata    map[string]string
}
