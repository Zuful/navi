package radar

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getUsageSummaryTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_usage_summary",
			mcp.WithDescription("Get product usage summary for a customer including DAU, WAU, MAU, and top events."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Customer ID (used across analytics systems)"),
			),
			mcp.WithNumber("days",
				mcp.Description("Number of days to look back (default: 30)"),
			),
		),
		Handler: p.handleGetUsageSummary,
	}
}

func (p *Provider) handleGetUsageSummary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	days := 30
	if d, ok := req.GetArguments()["days"].(float64); ok && d > 0 {
		days = int(d)
	}

	summary, err := p.client.GetUsageSummary(ctx, customerID, days)
	if err != nil {
		p.logger.Warn("failed to get usage summary", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to get usage summary: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("## Product Usage Summary\n\n")
	sb.WriteString(fmt.Sprintf("**Customer ID:** `%s` | **Period:** Last %d days\n\n", customerID, days))

	sb.WriteString("### Engagement Metrics\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("| ------ | ----- |\n")
	sb.WriteString(fmt.Sprintf("| Daily Active Users (DAU) | %d |\n", summary.DAU))
	sb.WriteString(fmt.Sprintf("| Weekly Active Users (WAU) | %d |\n", summary.WAU))
	sb.WriteString(fmt.Sprintf("| Monthly Active Users (MAU) | %d |\n", summary.MAU))
	sb.WriteString(fmt.Sprintf("| Total Events | %d |\n", summary.TotalEvents))

	if len(summary.TopEvents) > 0 {
		sb.WriteString("\n### Top Events\n\n")
		sb.WriteString("| Event | Count |\n")
		sb.WriteString("| ----- | ----- |\n")
		for _, ev := range summary.TopEvents {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", ev.Name, ev.Count))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}
