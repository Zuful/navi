package radar

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGetFeatureAdoption_HappyPath(t *testing.T) {
	mock := &mockUsageClient{
		adoption: &FeatureAdoption{
			CustomerID: "C-100",
			Features: []FeatureUsage{
				{Name: "Dashboard", EventCount: 500, UniqueUsers: 25, LastUsed: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
				{Name: "Reports", EventCount: 200, UniqueUsers: 10, LastUsed: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)},
			},
		},
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetFeatureAdoption(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Feature Adoption Report") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "Dashboard") {
		t.Error("missing feature name")
	}
	if !strings.Contains(text, "500") {
		t.Error("missing event count")
	}
	if !strings.Contains(text, "25") {
		t.Error("missing unique users")
	}
	if !strings.Contains(text, "2025-01-15") {
		t.Error("missing last used date")
	}
}

func TestGetFeatureAdoption_NoFeatures(t *testing.T) {
	mock := &mockUsageClient{
		adoption: &FeatureAdoption{
			CustomerID: "C-100",
			Features:   nil,
		},
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetFeatureAdoption(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "No feature usage data available") {
		t.Error("expected empty state message")
	}
}

func TestGetFeatureAdoption_MissingCustomerID(t *testing.T) {
	p := New(&mockUsageClient{})

	req := newReq(map[string]any{})
	result, err := p.handleGetFeatureAdoption(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "customer_id is required") {
		t.Error("expected customer_id required error")
	}
}

func TestGetFeatureAdoption_ClientError(t *testing.T) {
	mock := &mockUsageClient{
		adoptErr: errors.New("timeout"),
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetFeatureAdoption(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "failed to get feature adoption") {
		t.Error("expected error message in result")
	}
}
