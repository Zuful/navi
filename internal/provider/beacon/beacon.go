// Package beacon exposes support-ticket-related MCP tools.
package beacon

import (
	"fmt"
	"log/slog"

	"github.com/Zuful/navi/internal/httpclient"
	"github.com/Zuful/navi/internal/provider"
)

// Provider exposes support ticket tools via a pluggable SupportClient.
type Provider struct {
	client SupportClient
	logger *slog.Logger
}

// Option configures the beacon provider.
type Option func(*options)

type options struct {
	logger *slog.Logger
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// New creates a new beacon provider with the given support client.
func New(client SupportClient, opts ...Option) *Provider {
	o := &options{logger: slog.Default()}
	for _, fn := range opts {
		fn(o)
	}
	return &Provider{
		client: client,
		logger: o.logger.With(slog.String("provider", "beacon")),
	}
}

// NewFromConfig creates a beacon provider by selecting the backend from config.
func NewFromConfig(backend, apiKey, subdomain string, httpClient *httpclient.Client, opts ...Option) (*Provider, error) {
	var client SupportClient

	switch backend {
	case "zendesk", "":
		if apiKey == "" {
			return nil, fmt.Errorf("beacon: NAVI_BEACON_API_KEY is required for zendesk backend")
		}
		if subdomain == "" {
			return nil, fmt.Errorf("beacon: NAVI_BEACON_SUBDOMAIN is required for zendesk backend")
		}
		client = NewZendeskClient(apiKey, subdomain, httpClient)
	default:
		return nil, fmt.Errorf("beacon: unsupported backend %q", backend)
	}

	return New(client, opts...), nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "beacon" }

// Tools returns the tool definitions offered by this provider.
func (p *Provider) Tools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		p.getOpenTicketsTool(),
		p.getTicketHistoryTool(),
		p.getSatisfactionScoresTool(),
	}
}
