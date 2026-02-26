package pulse

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) listAtRiskAccountsTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("list_at_risk_accounts",
			mcp.WithDescription("List accounts with warning signals across billing and communications providers. Checks for past-due payments, cancellations, and lack of recent engagement."),
			mcp.WithNumber("days_ahead",
				mcp.Description("Number of days to look ahead for renewals (default: 30)"),
			),
		),
		Handler: p.handleListAtRiskAccounts,
	}
}

type atRiskAccount struct {
	customerID   string
	customerName string
	risks        []string
	severity     string // critical, warning, info
}

func (p *Provider) handleListAtRiskAccounts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	daysAhead := 30
	if d, ok := req.GetArguments()["days_ahead"].(float64); ok && d > 0 {
		daysAhead = int(d)
	}

	var accounts []atRiskAccount

	// Check billing signals.
	if p.billing != nil {
		renewals, err := p.billing.GetRenewalCalendar(ctx, daysAhead)
		if err != nil {
			p.logger.Warn("failed to get renewal calendar for risk assessment", "error", err)
		} else {
			for _, r := range renewals {
				var risks []string
				severity := "info"

				if r.Status == "expiring" {
					risks = append(risks, "Subscription expiring")
					severity = "critical"
				}
				if r.Status == "trial_ending" {
					risks = append(risks, "Trial ending soon")
					severity = "warning"
				}

				if len(risks) > 0 {
					accounts = append(accounts, atRiskAccount{
						customerID:   r.CustomerID,
						customerName: r.CustomerName,
						risks:        risks,
						severity:     severity,
					})
				}
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("## At-Risk Accounts\n\n")

	if len(accounts) == 0 {
		if p.billing == nil && p.comms == nil {
			sb.WriteString("_No providers configured. Set up billing and/or communications providers to detect at-risk accounts._\n")
		} else {
			sb.WriteString("_No at-risk accounts detected._\n")
		}
		return mcp.NewToolResultText(sb.String()), nil
	}

	// Sort: critical first, then warning, then info.
	severityOrder := map[string]int{"critical": 0, "warning": 1, "info": 2}
	for i := 0; i < len(accounts); i++ {
		for j := i + 1; j < len(accounts); j++ {
			if severityOrder[accounts[j].severity] < severityOrder[accounts[i].severity] {
				accounts[i], accounts[j] = accounts[j], accounts[i]
			}
		}
	}

	sb.WriteString("| Customer | Severity | Risks |\n")
	sb.WriteString("| -------- | -------- | ----- |\n")

	for _, a := range accounts {
		sb.WriteString(fmt.Sprintf("| %s (`%s`) | %s | %s |\n",
			a.customerName, a.customerID,
			formatSeverity(a.severity),
			strings.Join(a.risks, "; "),
		))
	}

	critCount := 0
	warnCount := 0
	for _, a := range accounts {
		switch a.severity {
		case "critical":
			critCount++
		case "warning":
			warnCount++
		}
	}

	sb.WriteString(fmt.Sprintf("\n**Summary:** %d critical, %d warning, %d total accounts\n",
		critCount, warnCount, len(accounts)))

	return mcp.NewToolResultText(sb.String()), nil
}

func formatSeverity(s string) string {
	switch s {
	case "critical":
		return "Critical"
	case "warning":
		return "Warning"
	case "info":
		return "Info"
	default:
		return s
	}
}
