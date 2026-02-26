package radar

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGetUsageSummary_HappyPath(t *testing.T) {
	mock := &mockUsageClient{
		summary: &UsageSummary{
			CustomerID:  "C-100",
			DAU:         50,
			WAU:         200,
			MAU:         500,
			TotalEvents: 10000,
			TopEvents: []EventCount{
				{Name: "page_view", Count: 5000},
				{Name: "button_click", Count: 3000},
			},
		},
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetUsageSummary(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Product Usage Summary") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "50") {
		t.Error("missing DAU value")
	}
	if !strings.Contains(text, "500") {
		t.Error("missing MAU value")
	}
	if !strings.Contains(text, "10000") {
		t.Error("missing total events")
	}
	if !strings.Contains(text, "page_view") {
		t.Error("missing top event name")
	}
}

func TestGetUsageSummary_ZeroUsage(t *testing.T) {
	mock := &mockUsageClient{
		summary: &UsageSummary{
			CustomerID:  "C-100",
			DAU:         0,
			WAU:         0,
			MAU:         0,
			TotalEvents: 0,
		},
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetUsageSummary(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Product Usage Summary") {
		t.Error("missing header for zero usage")
	}
}

func TestGetUsageSummary_MissingCustomerID(t *testing.T) {
	p := New(&mockUsageClient{})

	req := newReq(map[string]any{})
	result, err := p.handleGetUsageSummary(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "customer_id is required") {
		t.Error("expected customer_id required error")
	}
}

func TestGetUsageSummary_ClientError(t *testing.T) {
	mock := &mockUsageClient{
		summaryErr: errors.New("connection refused"),
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetUsageSummary(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "failed to get usage summary") {
		t.Error("expected error message in result")
	}
}
