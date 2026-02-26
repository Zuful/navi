package beacon

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getSatisfactionScoresTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_satisfaction_scores",
			mcp.WithDescription("Get CSAT scores and resolution metrics for a customer's support tickets."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Zendesk requester/user ID"),
			),
		),
		Handler: p.handleGetSatisfactionScores,
	}
}

func (p *Provider) handleGetSatisfactionScores(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	metrics, err := p.client.GetSatisfactionScores(ctx, customerID)
	if err != nil {
		p.logger.Error("failed to get satisfaction scores", "customer_id", customerID, "error", err)
		return mcp.NewToolResultError("Failed to retrieve satisfaction scores. Please check the customer ID and try again."), nil
	}

	var sb strings.Builder
	sb.WriteString("## Customer Satisfaction Metrics\n\n")
	sb.WriteString(fmt.Sprintf("**Customer ID:** `%s`\n\n", customerID))

	sb.WriteString("### Scores\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("| ------ | ----- |\n")

	if metrics.TotalRatings > 0 {
		sb.WriteString(fmt.Sprintf("| Average CSAT | %.0f%% |\n", metrics.AverageCSAT))
	} else {
		sb.WriteString("| Average CSAT | _No ratings_ |\n")
	}
	sb.WriteString(fmt.Sprintf("| Total Ratings | %d |\n", metrics.TotalRatings))

	sb.WriteString("\n### Ticket Load\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("| ------ | ----- |\n")
	sb.WriteString(fmt.Sprintf("| Open Tickets | %d |\n", metrics.OpenCount))
	sb.WriteString(fmt.Sprintf("| Pending Tickets | %d |\n", metrics.PendingCount))

	sb.WriteString("\n### Resolution Performance\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("| ------ | ----- |\n")
	if metrics.AverageResolutionHours > 0 {
		sb.WriteString(fmt.Sprintf("| Avg Resolution Time | %.1f hours |\n", metrics.AverageResolutionHours))
		sb.WriteString(fmt.Sprintf("| Avg First Response | %.1f hours |\n", metrics.FirstResponseHours))
	} else {
		sb.WriteString("| Avg Resolution Time | _No resolved tickets_ |\n")
		sb.WriteString("| Avg First Response | _No resolved tickets_ |\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}
