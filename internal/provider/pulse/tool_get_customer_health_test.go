package pulse

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider/beacon"
	"github.com/Zuful/navi/internal/provider/chronicle"
	"github.com/Zuful/navi/internal/provider/vault"
)

// --- Mock clients ---

type mockBillingClient struct {
	subscription *vault.Subscription
	subErr       error
	revenue      *vault.RevenueMetrics
	revErr       error
	renewals     []vault.Renewal
	renewErr     error
}

func (m *mockBillingClient) GetSubscriptionStatus(_ context.Context, _ string) (*vault.Subscription, error) {
	return m.subscription, m.subErr
}

func (m *mockBillingClient) GetRevenueMetrics(_ context.Context, _ string, _ int) (*vault.RevenueMetrics, error) {
	return m.revenue, m.revErr
}

func (m *mockBillingClient) GetRenewalCalendar(_ context.Context, _ int) ([]vault.Renewal, error) {
	return m.renewals, m.renewErr
}

type mockCommsClient struct {
	comms    []chronicle.Communication
	commsErr error
	timeline []chronicle.TimelineEvent
	tlErr    error
}

func (m *mockCommsClient) GetRecentCommunications(_ context.Context, _ string, _ int) ([]chronicle.Communication, error) {
	return m.comms, m.commsErr
}

func (m *mockCommsClient) GetContactTimeline(_ context.Context, _ string, _ int) ([]chronicle.TimelineEvent, error) {
	return m.timeline, m.tlErr
}

type mockSupportClient struct {
	openTickets  []beacon.Ticket
	openErr      error
	history      []beacon.Ticket
	historyErr   error
	satisfaction *beacon.SatisfactionMetrics
	satErr       error
}

func (m *mockSupportClient) GetOpenTickets(_ context.Context, _ string) ([]beacon.Ticket, error) {
	return m.openTickets, m.openErr
}

func (m *mockSupportClient) GetTicketHistory(_ context.Context, _ string, _ int) ([]beacon.Ticket, error) {
	return m.history, m.historyErr
}

func (m *mockSupportClient) GetSatisfactionScores(_ context.Context, _ string) (*beacon.SatisfactionMetrics, error) {
	return m.satisfaction, m.satErr
}

// --- Helpers ---

func newReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func getText(t *testing.T, result *mcp.CallToolResult) string {
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

// --- Tests ---

func TestGetCustomerHealth_SupportSignalPresent(t *testing.T) {
	support := &mockSupportClient{
		openTickets: make([]beacon.Ticket, 3), // 3 tickets → score 80
	}
	p := New(WithSupport(support))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetCustomerHealth(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Support Ticket Load") {
		t.Error("missing support signal row")
	}
	if !strings.Contains(text, "3 open/pending tickets") {
		t.Error("missing ticket count detail")
	}
	if !strings.Contains(text, "60/100") {
		t.Errorf("expected score 60 for 3 tickets, got: %s", text)
	}
}

func TestGetCustomerHealth_SupportSignalError(t *testing.T) {
	support := &mockSupportClient{
		openErr: errors.New("connection refused"),
	}
	p := New(WithSupport(support))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetCustomerHealth(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Unavailable") {
		t.Error("expected 'Unavailable' for support error")
	}
}

func TestGetCustomerHealth_SupportNotConfigured(t *testing.T) {
	p := New() // no support client

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetCustomerHealth(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Provider not configured") {
		t.Error("expected 'Provider not configured' when support is nil")
	}
}

func TestGetCustomerHealth_AllThreeSignals(t *testing.T) {
	billing := &mockBillingClient{
		subscription: &vault.Subscription{Status: "active"},       // score 100
		revenue:      &vault.RevenueMetrics{PaymentSuccessRate: 100}, // score 100
	}
	support := &mockSupportClient{
		openTickets: []beacon.Ticket{}, // 0 tickets → score 100
	}
	comms := &mockCommsClient{
		comms: []chronicle.Communication{
			{CreatedAt: time.Now().Add(-24 * time.Hour)}, // 1 day ago → score 100
		},
	}
	p := New(WithBilling(billing), WithSupport(support), WithComms(comms))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetCustomerHealth(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	// All signals score 100, average = 100
	if !strings.Contains(text, "100/100") {
		t.Errorf("expected overall score 100/100, got: %s", text)
	}
	if !strings.Contains(text, "Healthy") {
		t.Error("expected 'Healthy' label")
	}
}

func TestGetCustomerHealth_NoSignals(t *testing.T) {
	p := New() // no clients configured

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetCustomerHealth(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "N/A") {
		t.Error("expected N/A when no signals configured")
	}
}
