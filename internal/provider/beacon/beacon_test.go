package beacon

import (
	"context"
	"testing"
)

// mockSupportClient implements SupportClient for testing.
type mockSupportClient struct {
	openTickets  []Ticket
	openErr      error
	history      []Ticket
	historyErr   error
	satisfaction *SatisfactionMetrics
	satErr       error
}

func (m *mockSupportClient) GetOpenTickets(_ context.Context, _ string) ([]Ticket, error) {
	return m.openTickets, m.openErr
}

func (m *mockSupportClient) GetTicketHistory(_ context.Context, _ string, _ int) ([]Ticket, error) {
	return m.history, m.historyErr
}

func (m *mockSupportClient) GetSatisfactionScores(_ context.Context, _ string) (*SatisfactionMetrics, error) {
	return m.satisfaction, m.satErr
}

func TestNew(t *testing.T) {
	mock := &mockSupportClient{}
	p := New(mock)

	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.client != mock {
		t.Error("expected client to be the mock")
	}
}

func TestNewFromConfig_ZendeskRequiresAPIKey(t *testing.T) {
	_, err := NewFromConfig("zendesk", "", "subdomain", nil)
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
}

func TestNewFromConfig_ZendeskRequiresSubdomain(t *testing.T) {
	_, err := NewFromConfig("zendesk", "key", "", nil)
	if err == nil {
		t.Fatal("expected error when subdomain is empty")
	}
}

func TestNewFromConfig_UnsupportedBackend(t *testing.T) {
	_, err := NewFromConfig("jira", "key", "sub", nil)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}

func TestName(t *testing.T) {
	p := New(&mockSupportClient{})
	if got := p.Name(); got != "beacon" {
		t.Errorf("Name() = %q, want %q", got, "beacon")
	}
}

func TestTools_ReturnsThreeDefinitions(t *testing.T) {
	p := New(&mockSupportClient{})
	tools := p.Tools()
	if len(tools) != 3 {
		t.Fatalf("Tools() returned %d definitions, want 3", len(tools))
	}

	names := map[string]bool{}
	for _, td := range tools {
		names[td.Tool.Name] = true
	}
	for _, want := range []string{"get_open_tickets", "get_ticket_history", "get_satisfaction_scores"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}
