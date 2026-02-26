package radar

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getUsageTrendTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_usage_trend",
			mcp.WithDescription("Get product usage trends for a customer over time to identify growth or decline patterns."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Customer ID (used across analytics systems)"),
			),
			mcp.WithNumber("months",
				mcp.Description("Number of months to analyze (default: 3)"),
			),
		),
		Handler: p.handleGetUsageTrend,
	}
}

func (p *Provider) handleGetUsageTrend(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	months := 3
	if m, ok := req.GetArguments()["months"].(float64); ok && m > 0 {
		months = int(m)
	}

	trend, err := p.client.GetUsageTrend(ctx, customerID, months)
	if err != nil {
		p.logger.Warn("failed to get usage trend", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to get usage trend: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("## Product Usage Trend\n\n")
	sb.WriteString(fmt.Sprintf("**Customer ID:** `%s` | **Period:** Last %d months\n\n", customerID, months))
	sb.WriteString(fmt.Sprintf("**Trend:** %s (%.1f%% change)\n\n", trend.Direction, trend.ChangePercent))

	if len(trend.DataPoints) > 0 {
		sb.WriteString("| Period | Active Users | Event Count |\n")
		sb.WriteString("| ------ | ------------ | ----------- |\n")
		for _, dp := range trend.DataPoints {
			sb.WriteString(fmt.Sprintf("| %s | %d | %d |\n", dp.Period, dp.ActiveUsers, dp.EventCount))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}
