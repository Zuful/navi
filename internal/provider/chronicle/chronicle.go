// Package chronicle exposes communications-related MCP tools.
package chronicle

import (
	"fmt"
	"log/slog"

	"github.com/Zuful/navi/internal/httpclient"
	"github.com/Zuful/navi/internal/provider"
)

// Provider exposes communications tools via a pluggable CommsClient.
type Provider struct {
	client CommsClient
	logger *slog.Logger
}

// Option configures the chronicle provider.
type Option func(*options)

type options struct {
	logger *slog.Logger
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// New creates a new chronicle provider with the given comms client.
func New(client CommsClient, opts ...Option) *Provider {
	o := &options{logger: slog.Default()}
	for _, fn := range opts {
		fn(o)
	}
	return &Provider{
		client: client,
		logger: o.logger.With(slog.String("provider", "chronicle")),
	}
}

// NewFromConfig creates a chronicle provider by selecting the backend from config.
func NewFromConfig(backend, apiKey string, httpClient *httpclient.Client, opts ...Option) (*Provider, error) {
	var client CommsClient

	switch backend {
	case "hubspot", "":
		if apiKey == "" {
			return nil, fmt.Errorf("chronicle: NAVI_CHRONICLE_API_KEY is required for hubspot backend")
		}
		client = NewHubSpotClient(apiKey, httpClient)
	default:
		return nil, fmt.Errorf("chronicle: unsupported backend %q", backend)
	}

	return New(client, opts...), nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "chronicle" }

// Tools returns the tool definitions offered by this provider.
func (p *Provider) Tools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		p.getRecentCommunicationsTool(),
		p.getContactTimelineTool(),
	}
}
