// Package pulse exposes customer health scoring tools that aggregate signals
// from other providers (billing, communications).
package pulse

import (
	"log/slog"

	"github.com/Zuful/navi/internal/provider"
	"github.com/Zuful/navi/internal/provider/beacon"
	"github.com/Zuful/navi/internal/provider/chronicle"
	"github.com/Zuful/navi/internal/provider/radar"
	"github.com/Zuful/navi/internal/provider/vault"
)

// Provider exposes health-scoring aggregator tools.
type Provider struct {
	billing vault.BillingClient
	comms   chronicle.CommsClient
	support beacon.SupportClient
	usage   radar.UsageClient
	logger  *slog.Logger
}

// Option configures the pulse provider.
type Option func(*options)

type options struct {
	logger  *slog.Logger
	billing vault.BillingClient
	comms   chronicle.CommsClient
	support beacon.SupportClient
	usage   radar.UsageClient
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithBilling injects a billing client for health scoring.
func WithBilling(b vault.BillingClient) Option {
	return func(o *options) { o.billing = b }
}

// WithComms injects a communications client for health scoring.
func WithComms(c chronicle.CommsClient) Option {
	return func(o *options) { o.comms = c }
}

// WithSupport injects a support ticket client for health scoring.
func WithSupport(s beacon.SupportClient) Option {
	return func(o *options) { o.support = s }
}

// WithUsage injects a usage analytics client for health scoring.
func WithUsage(u radar.UsageClient) Option {
	return func(o *options) { o.usage = u }
}

// New creates a new pulse provider.
func New(opts ...Option) *Provider {
	o := &options{logger: slog.Default()}
	for _, fn := range opts {
		fn(o)
	}
	return &Provider{
		billing: o.billing,
		comms:   o.comms,
		support: o.support,
		usage:   o.usage,
		logger:  o.logger.With(slog.String("provider", "pulse")),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string { return "pulse" }

// Tools returns the tool definitions offered by this provider.
func (p *Provider) Tools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		p.getCustomerHealthTool(),
		p.listAtRiskAccountsTool(),
	}
}
