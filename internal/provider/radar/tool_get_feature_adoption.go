package radar

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getFeatureAdoptionTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_feature_adoption",
			mcp.WithDescription("Get feature adoption details for a customer showing which product features are being used."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Customer ID (used across analytics systems)"),
			),
			mcp.WithNumber("days",
				mcp.Description("Number of days to look back (default: 30)"),
			),
		),
		Handler: p.handleGetFeatureAdoption,
	}
}

func (p *Provider) handleGetFeatureAdoption(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	days := 30
	if d, ok := req.GetArguments()["days"].(float64); ok && d > 0 {
		days = int(d)
	}

	adoption, err := p.client.GetFeatureAdoption(ctx, customerID, days)
	if err != nil {
		p.logger.Warn("failed to get feature adoption", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("failed to get feature adoption: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("## Feature Adoption Report\n\n")
	sb.WriteString(fmt.Sprintf("**Customer ID:** `%s` | **Period:** Last %d days\n\n", customerID, days))

	if len(adoption.Features) == 0 {
		sb.WriteString("_No feature usage data available for this period._\n")
		return mcp.NewToolResultText(sb.String()), nil
	}

	sb.WriteString("| Feature | Event Count | Unique Users | Last Used |\n")
	sb.WriteString("| ------- | ----------- | ------------ | --------- |\n")
	for _, f := range adoption.Features {
		lastUsed := "N/A"
		if !f.LastUsed.IsZero() {
			lastUsed = f.LastUsed.Format("2006-01-02")
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %s |\n", f.Name, f.EventCount, f.UniqueUsers, lastUsed))
	}

	return mcp.NewToolResultText(sb.String()), nil
}
