// Command navi starts the Navi Customer Success MCP server over STDIO transport.
//
// Logs are written as structured JSON to stderr (the STDIO MCP transport
// uses stdin/stdout for the JSON-RPC protocol).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"

	"github.com/Zuful/navi/internal/config"
	"github.com/Zuful/navi/internal/dispatcher"
	"github.com/Zuful/navi/internal/httpclient"
	"github.com/Zuful/navi/internal/provider/beacon"
	"github.com/Zuful/navi/internal/provider/chronicle"
	"github.com/Zuful/navi/internal/provider/pulse"
	"github.com/Zuful/navi/internal/provider/radar"
	"github.com/Zuful/navi/internal/provider/scout"
	"github.com/Zuful/navi/internal/provider/vault"
)

// version is set at build-time via -ldflags.
var version = "0.1.0"

func main() {
	// Load configuration first to get log level.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.Level(config.ParseLogLevel(cfg.LogLevel)),
	}))
	slog.SetDefault(logger)

	if err := run(cfg, logger); err != nil {
		logger.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *slog.Logger) error {
	// ── Context with signal-based cancellation ──────────────────────
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	// ── Shared HTTP client ──────────────────────────────────────────
	cache := httpclient.NewCache(cfg.Cache.DefaultTTL, cfg.Cache.MaxEntries)
	httpClient := httpclient.New(
		httpclient.WithCache(cache),
		httpclient.WithRateLimit(10, 20),
	)

	// ── MCP server ──────────────────────────────────────────────────
	mcpSrv := server.NewMCPServer(
		"navi",
		version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	// ── Dispatcher ──────────────────────────────────────────────────
	d := dispatcher.New(mcpSrv, logger)

	// ── Track client interfaces for aggregators ─────────────────────
	var billingClient vault.BillingClient
	var commsClient chronicle.CommsClient
	var supportClient beacon.SupportClient
	var usageClient radar.UsageClient

	// ── Vault (billing) — optional ──────────────────────────────────
	if cfg.Vault != nil {
		vaultProv, err := vault.NewFromConfig(cfg.Vault.Backend, cfg.Vault.APIKey, httpClient, vault.WithLogger(logger))
		if err != nil {
			logger.Warn("vault provider not available, skipping",
				slog.String("reason", err.Error()),
			)
		} else {
			if err := d.Register(vaultProv); err != nil {
				return fmt.Errorf("register vault provider: %w", err)
			}
			// Extract the billing client for aggregators.
			billingClient = newBillingClientFromConfig(cfg.Vault.Backend, cfg.Vault.APIKey, httpClient)
		}
	} else {
		logger.Info("vault provider not configured, skipping")
	}

	// ── Chronicle (communications) — optional ───────────────────────
	if cfg.Chronicle != nil {
		chronicleProv, err := chronicle.NewFromConfig(cfg.Chronicle.Backend, cfg.Chronicle.APIKey, httpClient, chronicle.WithLogger(logger))
		if err != nil {
			logger.Warn("chronicle provider not available, skipping",
				slog.String("reason", err.Error()),
			)
		} else {
			if err := d.Register(chronicleProv); err != nil {
				return fmt.Errorf("register chronicle provider: %w", err)
			}
			commsClient = newCommsClientFromConfig(cfg.Chronicle.Backend, cfg.Chronicle.APIKey, httpClient)
		}
	} else {
		logger.Info("chronicle provider not configured, skipping")
	}

	// ── Beacon (support tickets) — optional ────────────────────────
	if cfg.Beacon != nil {
		beaconProv, err := beacon.NewFromConfig(cfg.Beacon.Backend, cfg.Beacon.APIKey, cfg.Beacon.Subdomain, httpClient, beacon.WithLogger(logger))
		if err != nil {
			logger.Warn("beacon provider not available, skipping",
				slog.String("reason", err.Error()),
			)
		} else {
			if err := d.Register(beaconProv); err != nil {
				return fmt.Errorf("register beacon provider: %w", err)
			}
			supportClient = newSupportClientFromConfig(cfg.Beacon.Backend, cfg.Beacon.APIKey, cfg.Beacon.Subdomain, httpClient)
		}
	} else {
		logger.Info("beacon provider not configured, skipping")
	}

	// ── Radar (product usage analytics) — optional ─────────────────
	if cfg.Radar != nil {
		radarProv, err := radar.NewFromConfig(cfg.Radar.Backend, cfg.Radar.APIKey, cfg.Radar.ProjectID, httpClient, radar.WithLogger(logger))
		if err != nil {
			logger.Warn("radar provider not available, skipping",
				slog.String("reason", err.Error()),
			)
		} else {
			if err := d.Register(radarProv); err != nil {
				return fmt.Errorf("register radar provider: %w", err)
			}
			usageClient = newUsageClientFromConfig(cfg.Radar.Backend, cfg.Radar.APIKey, cfg.Radar.ProjectID, httpClient)
		}
	} else {
		logger.Info("radar provider not configured, skipping")
	}

	// ── Pulse (health scoring aggregator) — always register ─────────
	pulseOpts := []pulse.Option{pulse.WithLogger(logger)}
	if billingClient != nil {
		pulseOpts = append(pulseOpts, pulse.WithBilling(billingClient))
	}
	if commsClient != nil {
		pulseOpts = append(pulseOpts, pulse.WithComms(commsClient))
	}
	if supportClient != nil {
		pulseOpts = append(pulseOpts, pulse.WithSupport(supportClient))
	}
	if usageClient != nil {
		pulseOpts = append(pulseOpts, pulse.WithUsage(usageClient))
	}
	pulseProv := pulse.New(pulseOpts...)
	if err := d.Register(pulseProv); err != nil {
		return fmt.Errorf("register pulse provider: %w", err)
	}

	// ── Scout (churn prediction aggregator) — always register ───────
	scoutOpts := []scout.Option{scout.WithLogger(logger)}
	if billingClient != nil {
		scoutOpts = append(scoutOpts, scout.WithBilling(billingClient))
	}
	if commsClient != nil {
		scoutOpts = append(scoutOpts, scout.WithComms(commsClient))
	}
	if supportClient != nil {
		scoutOpts = append(scoutOpts, scout.WithSupport(supportClient))
	}
	if usageClient != nil {
		scoutOpts = append(scoutOpts, scout.WithUsage(usageClient))
	}
	scoutProv := scout.New(scoutOpts...)
	if err := d.Register(scoutProv); err != nil {
		return fmt.Errorf("register scout provider: %w", err)
	}

	// ── STDIO transport ─────────────────────────────────────────────
	stdio := server.NewStdioServer(mcpSrv)

	logger.Info("navi starting",
		slog.String("transport", "stdio"),
		slog.String("version", version),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- stdio.Listen(ctx, os.Stdin, os.Stdout)
	}()

	select {
	case <-ctx.Done():
		logger.Info("received shutdown signal, exiting")
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("stdio listener: %w", err)
		}
		return nil
	}
}

// newBillingClientFromConfig creates a BillingClient for use by aggregators.
func newBillingClientFromConfig(backend, apiKey string, httpClient *httpclient.Client) vault.BillingClient {
	switch backend {
	case "stripe", "":
		return vault.NewStripeClient(apiKey, httpClient)
	default:
		return nil
	}
}

// newCommsClientFromConfig creates a CommsClient for use by aggregators.
func newCommsClientFromConfig(backend, apiKey string, httpClient *httpclient.Client) chronicle.CommsClient {
	switch backend {
	case "hubspot", "":
		return chronicle.NewHubSpotClient(apiKey, httpClient)
	default:
		return nil
	}
}

// newSupportClientFromConfig creates a SupportClient for use by aggregators.
func newSupportClientFromConfig(backend, apiKey, subdomain string, httpClient *httpclient.Client) beacon.SupportClient {
	switch backend {
	case "zendesk", "":
		return beacon.NewZendeskClient(apiKey, subdomain, httpClient)
	default:
		return nil
	}
}

// newUsageClientFromConfig creates a UsageClient for use by aggregators.
func newUsageClientFromConfig(backend, apiKey, projectID string, httpClient *httpclient.Client) radar.UsageClient {
	switch backend {
	case "mixpanel", "":
		return radar.NewMixpanelClient(apiKey, projectID, httpClient)
	default:
		return nil
	}
}
