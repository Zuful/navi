package beacon

import (
	"context"
	"errors"
	"strings"
	"testing"

)

func TestGetSatisfactionScores_HappyPath(t *testing.T) {
	mock := &mockSupportClient{
		satisfaction: &SatisfactionMetrics{
			CustomerID:             "C-100",
			AverageCSAT:            85,
			TotalRatings:           42,
			OpenCount:              3,
			PendingCount:           1,
			AverageResolutionHours: 12.5,
			FirstResponseHours:     2.3,
		},
	}
	p := New(mock)

	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetSatisfactionScores(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Customer Satisfaction Metrics") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "85%") {
		t.Error("missing CSAT percentage")
	}
	if !strings.Contains(text, "42") {
		t.Error("missing total ratings")
	}
	if !strings.Contains(text, "12.5 hours") {
		t.Error("missing resolution time")
	}
	if !strings.Contains(text, "2.3 hours") {
		t.Error("missing first response time")
	}
}

func TestGetSatisfactionScores_NoRatings(t *testing.T) {
	mock := &mockSupportClient{
		satisfaction: &SatisfactionMetrics{
			CustomerID:   "C-100",
			TotalRatings: 0,
			OpenCount:    1,
		},
	}
	p := New(mock)

	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetSatisfactionScores(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No ratings") {
		t.Error("expected 'No ratings' for zero TotalRatings")
	}
}

func TestGetSatisfactionScores_NoResolvedTickets(t *testing.T) {
	mock := &mockSupportClient{
		satisfaction: &SatisfactionMetrics{
			CustomerID:             "C-100",
			TotalRatings:           5,
			AverageCSAT:            70,
			AverageResolutionHours: 0,
		},
	}
	p := New(mock)

	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetSatisfactionScores(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No resolved tickets") {
		t.Error("expected 'No resolved tickets' when AverageResolutionHours is 0")
	}
}

func TestGetSatisfactionScores_MissingCustomerID(t *testing.T) {
	p := New(&mockSupportClient{})
	req := newCallToolRequest(map[string]any{})
	result, err := p.handleGetSatisfactionScores(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result")
	}
}

func TestGetSatisfactionScores_ClientError(t *testing.T) {
	mock := &mockSupportClient{satErr: errors.New("api error")}
	p := New(mock)
	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetSatisfactionScores(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Failed to retrieve satisfaction scores") {
		t.Errorf("unexpected error text: %s", text)
	}
}
