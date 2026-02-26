package scout

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider/beacon"
	"github.com/Zuful/navi/internal/provider/chronicle"
	"github.com/Zuful/navi/internal/provider/vault"
)

// --- Mock clients (same pattern as pulse tests) ---

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

func TestGetChurnRisk_HighTicketVolume(t *testing.T) {
	support := &mockSupportClient{
		openTickets:  make([]beacon.Ticket, 8), // >5 → High Ticket Volume
		satisfaction: &beacon.SatisfactionMetrics{TotalRatings: 0},
	}
	p := New(WithSupport(support))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "High Ticket Volume") {
		t.Error("expected 'High Ticket Volume' factor")
	}
	if !strings.Contains(text, "8 open/pending support tickets") {
		t.Error("expected ticket count in details")
	}
}

func TestGetChurnRisk_LowCSAT(t *testing.T) {
	support := &mockSupportClient{
		openTickets: []beacon.Ticket{},
		satisfaction: &beacon.SatisfactionMetrics{
			TotalRatings: 10,
			AverageCSAT:  35, // <50% → Low CSAT Score
		},
	}
	p := New(WithSupport(support))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Low CSAT Score") {
		t.Error("expected 'Low CSAT Score' factor")
	}
}

func TestGetChurnRisk_SlowResolution(t *testing.T) {
	support := &mockSupportClient{
		openTickets: []beacon.Ticket{},
		satisfaction: &beacon.SatisfactionMetrics{
			TotalRatings:           5,
			AverageCSAT:            80,
			AverageResolutionHours: 72, // >48h → Slow Resolution Times
		},
	}
	p := New(WithSupport(support))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Slow Resolution Times") {
		t.Error("expected 'Slow Resolution Times' factor")
	}
	if !strings.Contains(text, "72.0 hour") {
		t.Error("expected resolution hours in details")
	}
}

func TestGetChurnRisk_SupportRecommendations(t *testing.T) {
	support := &mockSupportClient{
		openTickets:  make([]beacon.Ticket, 6), // >5 → triggers support issue
		satisfaction: &beacon.SatisfactionMetrics{TotalRatings: 0},
	}
	p := New(WithSupport(support))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Review open support tickets") {
		t.Error("expected support-related recommendation")
	}
	if !strings.Contains(text, "Escalate unresolved tickets") {
		t.Error("expected escalation recommendation")
	}
}

func TestGetChurnRisk_NormalizedRiskScore(t *testing.T) {
	// Support with high ticket volume: riskScore += 50 from 1 signal count
	// Plus satisfaction with low CSAT: riskScore += 40 (no extra signal count for sat)
	// The support.GetOpenTickets counts as 1 signal, so normalized = (50+40)/1
	// But that would be >100, capped at 100
	// Actually: signalCount is incremented per client call that succeeds:
	// - GetOpenTickets success → signalCount++ (1), riskScore += 50
	// - GetSatisfactionScores success (no increment), riskScore += 40
	// Total: 90/1 = 90, capped at 100 → shows 90
	support := &mockSupportClient{
		openTickets: make([]beacon.Ticket, 8),
		satisfaction: &beacon.SatisfactionMetrics{
			TotalRatings: 10,
			AverageCSAT:  30,
		},
	}
	p := New(WithSupport(support))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "High Risk") {
		t.Error("expected 'High Risk' label for high support load + low CSAT")
	}
}

func TestGetChurnRisk_NoSignals(t *testing.T) {
	p := New() // no clients

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "N/A") {
		t.Error("expected N/A when no signals")
	}
}
