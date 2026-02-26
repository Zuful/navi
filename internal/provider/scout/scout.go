// Package scout exposes churn prediction and expansion opportunity tools.
package scout

import (
	"log/slog"

	"github.com/Zuful/navi/internal/provider"
	"github.com/Zuful/navi/internal/provider/chronicle"
	"github.com/Zuful/navi/internal/provider/vault"
)

// Provider exposes churn risk and expansion opportunity tools.
type Provider struct {
	billing vault.BillingClient
	comms   chronicle.CommsClient
	logger  *slog.Logger
}

// Option configures the scout provider.
type Option func(*options)

type options struct {
	logger  *slog.Logger
	billing vault.BillingClient
	comms   chronicle.CommsClient
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithBilling injects a billing client.
func WithBilling(b vault.BillingClient) Option {
	return func(o *options) { o.billing = b }
}

// WithComms injects a communications client.
func WithComms(c chronicle.CommsClient) Option {
	return func(o *options) { o.comms = c }
}

// New creates a new scout provider.
func New(opts ...Option) *Provider {
	o := &options{logger: slog.Default()}
	for _, fn := range opts {
		fn(o)
	}
	return &Provider{
		billing: o.billing,
		comms:   o.comms,
		logger:  o.logger.With(slog.String("provider", "scout")),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string { return "scout" }

// Tools returns the tool definitions offered by this provider.
func (p *Provider) Tools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		p.getChurnRiskTool(),
	}
}
