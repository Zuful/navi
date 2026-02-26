package beacon

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getTicketHistoryTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_ticket_history",
			mcp.WithDescription("Get full ticket history for a customer across all statuses."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Zendesk requester/user ID"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of tickets to return (default: 50)"),
			),
		),
		Handler: p.handleGetTicketHistory,
	}
}

func (p *Provider) handleGetTicketHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	limit := 50
	if l, ok := req.GetArguments()["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	tickets, err := p.client.GetTicketHistory(ctx, customerID, limit)
	if err != nil {
		p.logger.Error("failed to get ticket history", "customer_id", customerID, "error", err)
		return mcp.NewToolResultError("Failed to retrieve ticket history. Please check the customer ID and try again."), nil
	}

	var sb strings.Builder
	sb.WriteString("## Ticket History\n\n")

	if len(tickets) == 0 {
		sb.WriteString("_No tickets found for this customer._\n")
		return mcp.NewToolResultText(sb.String()), nil
	}

	sb.WriteString(fmt.Sprintf("Showing %d ticket(s):\n\n", len(tickets)))

	sb.WriteString("| ID | Subject | Status | Priority | Created | Resolved |\n")
	sb.WriteString("| -- | ------- | ------ | -------- | ------- | -------- |\n")

	for _, t := range tickets {
		resolved := "-"
		if t.ResolvedAt != nil {
			resolved = t.ResolvedAt.Format("2006-01-02")
		}
		priority := "-"
		if t.Priority != "" {
			priority = formatPriority(t.Priority)
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s |\n",
			t.ID,
			t.Subject,
			formatStatus(t.Status),
			priority,
			t.CreatedAt.Format("2006-01-02"),
			resolved,
		))
	}

	// Summary by status.
	statusCounts := make(map[string]int)
	for _, t := range tickets {
		statusCounts[t.Status]++
	}

	sb.WriteString("\n**Summary:** ")
	parts := []string{}
	for _, status := range []string{"open", "pending", "hold", "solved", "closed"} {
		if count, ok := statusCounts[status]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", count, status))
		}
	}
	sb.WriteString(strings.Join(parts, ", "))
	sb.WriteString("\n")

	return mcp.NewToolResultText(sb.String()), nil
}
