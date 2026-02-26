package radar

import (
	"fmt"
	"log/slog"

	"github.com/Zuful/navi/internal/httpclient"
	"github.com/Zuful/navi/internal/provider"
)

// Provider exposes product usage analytics tools via a pluggable UsageClient.
type Provider struct {
	client UsageClient
	logger *slog.Logger
}

// Option configures the radar provider.
type Option func(*options)

type options struct {
	logger *slog.Logger
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// New creates a new radar provider with the given usage client.
func New(client UsageClient, opts ...Option) *Provider {
	o := &options{logger: slog.Default()}
	for _, fn := range opts {
		fn(o)
	}
	return &Provider{
		client: client,
		logger: o.logger.With(slog.String("provider", "radar")),
	}
}

// NewFromConfig creates a radar provider by selecting the backend from config.
func NewFromConfig(backend, apiKey, projectID string, httpClient *httpclient.Client, opts ...Option) (*Provider, error) {
	var client UsageClient

	switch backend {
	case "mixpanel", "":
		if apiKey == "" {
			return nil, fmt.Errorf("radar: NAVI_RADAR_API_KEY is required for mixpanel backend")
		}
		if projectID == "" {
			return nil, fmt.Errorf("radar: NAVI_RADAR_PROJECT_ID is required for mixpanel backend")
		}
		client = NewMixpanelClient(apiKey, projectID, httpClient)
	default:
		return nil, fmt.Errorf("radar: unsupported backend %q", backend)
	}

	return New(client, opts...), nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "radar" }

// Tools returns the tool definitions offered by this provider.
func (p *Provider) Tools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		p.getUsageSummaryTool(),
		p.getFeatureAdoptionTool(),
		p.getUsageTrendTool(),
	}
}
