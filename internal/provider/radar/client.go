// Package radar exposes product usage analytics tools (Mixpanel).
package radar

import (
	"context"
	"time"
)

// UsageClient defines the interface for product usage analytics backends.
type UsageClient interface {
	GetUsageSummary(ctx context.Context, customerID string, days int) (*UsageSummary, error)
	GetFeatureAdoption(ctx context.Context, customerID string, days int) (*FeatureAdoption, error)
	GetUsageTrend(ctx context.Context, customerID string, months int) (*UsageTrend, error)
}

// UsageSummary contains high-level engagement metrics for a customer.
type UsageSummary struct {
	CustomerID  string
	DAU         int // Daily Active Users
	WAU         int // Weekly Active Users
	MAU         int // Monthly Active Users
	TotalEvents int
	TopEvents   []EventCount
}

// EventCount represents a named event and how many times it occurred.
type EventCount struct {
	Name  string
	Count int
}

// FeatureAdoption describes which product features a customer is using.
type FeatureAdoption struct {
	CustomerID string
	Features   []FeatureUsage
}

// FeatureUsage holds adoption details for a single feature.
type FeatureUsage struct {
	Name        string
	EventCount  int
	UniqueUsers int
	LastUsed    time.Time
}

// UsageTrend tracks usage over multiple periods.
type UsageTrend struct {
	CustomerID      string
	Direction       string  // "growing", "declining", "flat"
	ChangePercent   float64 // positive = growth, negative = decline
	DataPoints      []UsageDataPoint
}

// UsageDataPoint is a single period's usage data.
type UsageDataPoint struct {
	Period      string // e.g. "2024-01"
	ActiveUsers int
	EventCount  int
}
