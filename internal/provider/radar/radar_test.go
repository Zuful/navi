package radar

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// --- Mock client ---

type mockUsageClient struct {
	summary    *UsageSummary
	summaryErr error
	adoption   *FeatureAdoption
	adoptErr   error
	trend      *UsageTrend
	trendErr   error
}

func (m *mockUsageClient) GetUsageSummary(_ context.Context, _ string, _ int) (*UsageSummary, error) {
	return m.summary, m.summaryErr
}

func (m *mockUsageClient) GetFeatureAdoption(_ context.Context, _ string, _ int) (*FeatureAdoption, error) {
	return m.adoption, m.adoptErr
}

func (m *mockUsageClient) GetUsageTrend(_ context.Context, _ string, _ int) (*UsageTrend, error) {
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

// Ensure mockUsageClient satisfies the interface at compile time.
var _ UsageClient = (*mockUsageClient)(nil)

// Silence unused import for time in tests that need it.
var _ = time.Now

// --- Tests ---

func TestNew(t *testing.T) {
	mock := &mockUsageClient{}
	p := New(mock)
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.client != mock {
		t.Error("expected client to be the mock")
	}
}

func TestNewFromConfig_MissingAPIKey(t *testing.T) {
	_, err := NewFromConfig("mixpanel", "", "proj-123", nil)
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
}

func TestNewFromConfig_MissingProjectID(t *testing.T) {
	_, err := NewFromConfig("mixpanel", "key", "", nil)
	if err == nil {
		t.Fatal("expected error when project ID is empty")
	}
}

func TestNewFromConfig_UnsupportedBackend(t *testing.T) {
	_, err := NewFromConfig("amplitude", "key", "proj", nil)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}

func TestName(t *testing.T) {
	p := New(&mockUsageClient{})
	if got := p.Name(); got != "radar" {
		t.Errorf("Name() = %q, want %q", got, "radar")
	}
}

func TestTools_ReturnsThreeDefinitions(t *testing.T) {
	p := New(&mockUsageClient{})
	tools := p.Tools()
	if len(tools) != 3 {
		t.Fatalf("Tools() returned %d definitions, want 3", len(tools))
	}

	names := map[string]bool{}
	for _, td := range tools {
		names[td.Tool.Name] = true
	}
	for _, want := range []string{"get_usage_summary", "get_feature_adoption", "get_usage_trend"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}
