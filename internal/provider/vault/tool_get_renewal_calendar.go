package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getRenewalCalendarTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_renewal_calendar",
			mcp.WithDescription("Get upcoming subscription renewals and expirations within a specified number of days."),
			mcp.WithNumber("days_ahead",
				mcp.Description("Number of days to look ahead (default: 30)"),
			),
		),
		Handler: p.handleGetRenewalCalendar,
	}
}

func (p *Provider) handleGetRenewalCalendar(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	daysAhead := 30
	if d, ok := req.GetArguments()["days_ahead"].(float64); ok && d > 0 {
		daysAhead = int(d)
	}

	renewals, err := p.client.GetRenewalCalendar(ctx, daysAhead)
	if err != nil {
		p.logger.Error("failed to get renewal calendar", "days_ahead", daysAhead, "error", err)
		return mcp.NewToolResultError("Failed to retrieve renewal calendar."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Renewal Calendar (Next %d Days)\n\n", daysAhead))

	if len(renewals) == 0 {
		sb.WriteString("_No upcoming renewals or expirations._\n")
		return mcp.NewToolResultText(sb.String()), nil
	}

	sb.WriteString("| Customer | Plan | MRR | Renewal Date | Status |\n")
	sb.WriteString("| -------- | ---- | --- | ------------ | ------ |\n")

	for _, r := range renewals {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			r.CustomerName, r.Plan,
			formatCurrency(r.MRR, r.Currency),
			r.RenewalDate.Format("2006-01-02"),
			formatRenewalStatus(r.Status),
		))
	}

	sb.WriteString(fmt.Sprintf("\n**Total:** %d upcoming events\n", len(renewals)))

	return mcp.NewToolResultText(sb.String()), nil
}

func formatRenewalStatus(status string) string {
	switch status {
	case "renewing":
		return "Renewing"
	case "expiring":
		return "Expiring"
	case "trial_ending":
		return "Trial Ending"
	default:
		return status
	}
}
