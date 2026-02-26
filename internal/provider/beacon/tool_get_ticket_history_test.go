package beacon

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

)

func TestGetTicketHistory_HappyPath_DefaultLimit(t *testing.T) {
	resolved := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	mock := &mockSupportClient{
		history: []Ticket{
			{ID: "T-1", Subject: "Bug report", Status: "solved", Priority: "normal", CreatedAt: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC), ResolvedAt: &resolved},
			{ID: "T-2", Subject: "Feature request", Status: "open", Priority: "low", CreatedAt: time.Date(2025, 1, 12, 10, 0, 0, 0, time.UTC)},
		},
	}
	p := New(mock)

	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetTicketHistory(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Ticket History") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "T-1") {
		t.Error("missing ticket T-1")
	}
	if !strings.Contains(text, "2025-01-20") {
		t.Error("missing resolved date")
	}
	if !strings.Contains(text, "1 solved") {
		t.Error("missing solved count in summary")
	}
	if !strings.Contains(text, "1 open") {
		t.Error("missing open count in summary")
	}
}

func TestGetTicketHistory_CustomLimit(t *testing.T) {
	mock := &mockSupportClient{
		history: []Ticket{
			{ID: "T-1", Subject: "Test", Status: "open", Priority: "normal", CreatedAt: time.Now()},
		},
	}
	p := New(mock)

	req := newCallToolRequest(map[string]any{"customer_id": "C-100", "limit": float64(10)})
	result, err := p.handleGetTicketHistory(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("expected success result")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Showing 1 ticket(s)") {
		t.Error("expected showing count")
	}
}

func TestGetTicketHistory_Empty(t *testing.T) {
	p := New(&mockSupportClient{history: []Ticket{}})
	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetTicketHistory(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "No tickets found for this customer") {
		t.Error("expected empty message")
	}
}

func TestGetTicketHistory_MissingCustomerID(t *testing.T) {
	p := New(&mockSupportClient{})
	req := newCallToolRequest(map[string]any{})
	result, err := p.handleGetTicketHistory(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "customer_id is required") {
		t.Errorf("unexpected error text: %s", text)
	}
}

func TestGetTicketHistory_ClientError(t *testing.T) {
	mock := &mockSupportClient{historyErr: errors.New("timeout")}
	p := New(mock)
	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetTicketHistory(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Failed to retrieve ticket history") {
		t.Errorf("unexpected error text: %s", text)
	}
}
