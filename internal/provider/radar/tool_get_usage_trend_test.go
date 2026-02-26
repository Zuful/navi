package radar

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGetUsageTrend_Growing(t *testing.T) {
	mock := &mockUsageClient{
		trend: &UsageTrend{
			CustomerID:    "C-100",
			Direction:     "growing",
			ChangePercent: 25.5,
			DataPoints: []UsageDataPoint{
				{Period: "2025-01", ActiveUsers: 100, EventCount: 5000},
				{Period: "2025-02", ActiveUsers: 120, EventCount: 6000},
				{Period: "2025-03", ActiveUsers: 125, EventCount: 6500},
			},
		},
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetUsageTrend(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "growing") {
		t.Error("missing trend direction")
	}
	if !strings.Contains(text, "25.5%") {
		t.Error("missing change percent")
	}
	if !strings.Contains(text, "2025-01") {
		t.Error("missing data point period")
	}
}

func TestGetUsageTrend_Declining(t *testing.T) {
	mock := &mockUsageClient{
		trend: &UsageTrend{
			CustomerID:    "C-100",
			Direction:     "declining",
			ChangePercent: -15.0,
			DataPoints: []UsageDataPoint{
				{Period: "2025-01", ActiveUsers: 100, EventCount: 5000},
				{Period: "2025-02", ActiveUsers: 90, EventCount: 4500},
				{Period: "2025-03", ActiveUsers: 85, EventCount: 4250},
			},
		},
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetUsageTrend(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "declining") {
		t.Error("missing declining direction")
	}
}

func TestGetUsageTrend_Flat(t *testing.T) {
	mock := &mockUsageClient{
		trend: &UsageTrend{
			CustomerID:    "C-100",
			Direction:     "flat",
			ChangePercent: 0.5,
			DataPoints: []UsageDataPoint{
				{Period: "2025-01", ActiveUsers: 100, EventCount: 5000},
			},
		},
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetUsageTrend(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "flat") {
		t.Error("missing flat direction")
	}
}

func TestGetUsageTrend_MissingCustomerID(t *testing.T) {
	p := New(&mockUsageClient{})

	req := newReq(map[string]any{})
	result, err := p.handleGetUsageTrend(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "customer_id is required") {
		t.Error("expected customer_id required error")
	}
}

func TestGetUsageTrend_ClientError(t *testing.T) {
	mock := &mockUsageClient{
		trendErr: errors.New("service unavailable"),
	}
	p := New(mock)

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetUsageTrend(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "failed to get usage trend") {
		t.Error("expected error message in result")
	}
}
