package beacon

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func newCallToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	tc, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatal("expected TextContent")
	}
	return tc.Text
}

func TestGetOpenTickets_HappyPath(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	mock := &mockSupportClient{
		openTickets: []Ticket{
			{ID: "T-1", Subject: "Login broken", Status: "open", Priority: "urgent", Channel: "email", CreatedAt: now, AssignedTo: "Alice"},
			{ID: "T-2", Subject: "Slow dashboard", Status: "pending", Priority: "high", Channel: "chat", CreatedAt: now},
		},
	}
	p := New(mock)

	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetOpenTickets(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success result")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Open Support Tickets") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "T-1") {
		t.Error("missing ticket T-1")
	}
	if !strings.Contains(text, "T-2") {
		t.Error("missing ticket T-2")
	}
	if !strings.Contains(text, "1 urgent") {
		t.Error("missing urgent count in summary")
	}
	if !strings.Contains(text, "1 high") {
		t.Error("missing high count in summary")
	}
	if !strings.Contains(text, "Alice") {
		t.Error("missing assignee")
	}
}

func TestGetOpenTickets_Empty(t *testing.T) {
	p := New(&mockSupportClient{openTickets: []Ticket{}})
	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetOpenTickets(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "No open or pending tickets found") {
		t.Error("expected empty message")
	}
}

func TestGetOpenTickets_MissingCustomerID(t *testing.T) {
	p := New(&mockSupportClient{})

	tests := []struct {
		name string
		args map[string]any
	}{
		{"nil args", nil},
		{"empty string", map[string]any{"customer_id": ""}},
		{"wrong type", map[string]any{"customer_id": 123}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newCallToolRequest(tt.args)
			result, err := p.handleGetOpenTickets(context.Background(), req)
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
		})
	}
}

func TestGetOpenTickets_ClientError(t *testing.T) {
	mock := &mockSupportClient{openErr: errors.New("connection refused")}
	p := New(mock)

	req := newCallToolRequest(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetOpenTickets(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Failed to retrieve open tickets") {
		t.Errorf("unexpected error text: %s", text)
	}
}
