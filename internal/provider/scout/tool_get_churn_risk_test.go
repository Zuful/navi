package scout

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider/beacon"
	"github.com/Zuful/navi/internal/provider/chronicle"
	"github.com/Zuful/navi/internal/provider/radar"
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

type mockUsageClient struct {
	summary    *radar.UsageSummary
	summaryErr error
	adoption   *radar.FeatureAdoption
	adoptErr   error
	trend      *radar.UsageTrend
	trendErr   error
}

func (m *mockUsageClient) GetUsageSummary(_ context.Context, _ string, _ int) (*radar.UsageSummary, error) {
	return m.summary, m.summaryErr
}

func (m *mockUsageClient) GetFeatureAdoption(_ context.Context, _ string, _ int) (*radar.FeatureAdoption, error) {
	return m.adoption, m.adoptErr
}

func (m *mockUsageClient) GetUsageTrend(_ context.Context, _ string, _ int) (*radar.UsageTrend, error) {
	return m.trend, m.trendErr
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

func TestGetChurnRisk_DecliningUsage(t *testing.T) {
	usage := &mockUsageClient{
		trend: &radar.UsageTrend{
			Direction:     "declining",
			ChangePercent: -35.0, // < -30% → high impact
		},
		summary: &radar.UsageSummary{DAU: 5, MAU: 100},
	}
	p := New(WithUsage(usage))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Declining Usage") {
		t.Error("expected 'Declining Usage' factor")
	}
	if !strings.Contains(text, "High") {
		t.Error("expected high impact for >30% decline")
	}
}

func TestGetChurnRisk_UsageSlowdown(t *testing.T) {
	usage := &mockUsageClient{
		trend: &radar.UsageTrend{
			Direction:     "declining",
			ChangePercent: -15.0, // < -10% but > -30% → medium impact
		},
		summary: &radar.UsageSummary{DAU: 10, MAU: 100},
	}
	p := New(WithUsage(usage))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Usage Slowdown") {
		t.Error("expected 'Usage Slowdown' factor")
	}
	if !strings.Contains(text, "Medium") {
		t.Error("expected medium impact for 10-30% decline")
	}
}

func TestGetChurnRisk_NoRecentActivity(t *testing.T) {
	usage := &mockUsageClient{
		trend: &radar.UsageTrend{
			Direction:     "declining",
			ChangePercent: -5.0, // not enough for declining/slowdown
		},
		summary: &radar.UsageSummary{DAU: 0, MAU: 0}, // DAU=0 → No Recent Activity
	}
	p := New(WithUsage(usage))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "No Recent Activity") {
		t.Error("expected 'No Recent Activity' factor")
	}
}

func TestGetChurnRisk_UsageRecommendations(t *testing.T) {
	usage := &mockUsageClient{
		trend: &radar.UsageTrend{
			Direction:     "declining",
			ChangePercent: -40.0,
		},
		summary: &radar.UsageSummary{DAU: 5, MAU: 100},
	}
	p := New(WithUsage(usage))

	req := newReq(map[string]any{"customer_id": "C-100"})
	result, err := p.handleGetChurnRisk(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := getText(t, result)
	if !strings.Contains(text, "Review product usage patterns") {
		t.Error("expected usage-related recommendation")
	}
	if !strings.Contains(text, "Schedule a product training") {
		t.Error("expected training recommendation")
	}
}
