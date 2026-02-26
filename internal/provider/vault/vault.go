// Package vault exposes billing-related MCP tools.
package vault

import (
	"fmt"
	"log/slog"

	"github.com/Zuful/navi/internal/httpclient"
	"github.com/Zuful/navi/internal/provider"
)

// Provider exposes billing tools via a pluggable BillingClient.
type Provider struct {
	client BillingClient
	logger *slog.Logger
}

// Option configures the vault provider.
type Option func(*options)

type options struct {
	logger *slog.Logger
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// New creates a new vault provider with the given billing client.
func New(client BillingClient, opts ...Option) *Provider {
	o := &options{logger: slog.Default()}
	for _, fn := range opts {
		fn(o)
	}
	return &Provider{
		client: client,
		logger: o.logger.With(slog.String("provider", "vault")),
	}
}

// NewFromConfig creates a vault provider by selecting the backend from config.
func NewFromConfig(backend, apiKey string, httpClient *httpclient.Client, opts ...Option) (*Provider, error) {
	var client BillingClient

	switch backend {
	case "stripe", "":
		if apiKey == "" {
			return nil, fmt.Errorf("vault: NAVI_VAULT_API_KEY is required for stripe backend")
		}
		client = NewStripeClient(apiKey, httpClient)
	default:
		return nil, fmt.Errorf("vault: unsupported backend %q", backend)
	}

	return New(client, opts...), nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "vault" }

// Tools returns the tool definitions offered by this provider.
func (p *Provider) Tools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		p.getSubscriptionStatusTool(),
		p.getRevenueMetricsTool(),
		p.getRenewalCalendarTool(),
	}
}
