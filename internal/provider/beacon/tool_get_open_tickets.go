package beacon

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getOpenTicketsTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_open_tickets",
			mcp.WithDescription("List open and pending support tickets for a customer."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Zendesk requester/user ID"),
			),
		),
		Handler: p.handleGetOpenTickets,
	}
}

func (p *Provider) handleGetOpenTickets(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	tickets, err := p.client.GetOpenTickets(ctx, customerID)
	if err != nil {
		p.logger.Error("failed to get open tickets", "customer_id", customerID, "error", err)
		return mcp.NewToolResultError("Failed to retrieve open tickets. Please check the customer ID and try again."), nil
	}

	var sb strings.Builder
	sb.WriteString("## Open Support Tickets\n\n")

	if len(tickets) == 0 {
		sb.WriteString("_No open or pending tickets found._\n")
		return mcp.NewToolResultText(sb.String()), nil
	}

	sb.WriteString(fmt.Sprintf("Found %d open/pending ticket(s):\n\n", len(tickets)))

	sb.WriteString("| ID | Subject | Status | Priority | Channel | Created | Assigned To |\n")
	sb.WriteString("| -- | ------- | ------ | -------- | ------- | ------- | ----------- |\n")

	for _, t := range tickets {
		assignee := "-"
		if t.AssignedTo != "" {
			assignee = t.AssignedTo
		}
		priority := "-"
		if t.Priority != "" {
			priority = formatPriority(t.Priority)
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s | %s |\n",
			t.ID,
			t.Subject,
			formatStatus(t.Status),
			priority,
			t.Channel,
			t.CreatedAt.Format("2006-01-02"),
			assignee,
		))
	}

	// Summary by priority.
	urgentCount, highCount, normalCount, lowCount := 0, 0, 0, 0
	for _, t := range tickets {
		switch t.Priority {
		case "urgent":
			urgentCount++
		case "high":
			highCount++
		case "normal":
			normalCount++
		case "low":
			lowCount++
		}
	}
	sb.WriteString(fmt.Sprintf("\n**Summary:** %d urgent, %d high, %d normal, %d low\n",
		urgentCount, highCount, normalCount, lowCount))

	return mcp.NewToolResultText(sb.String()), nil
}

func formatStatus(s string) string {
	switch s {
	case "open":
		return "Open"
	case "pending":
		return "Pending"
	case "hold":
		return "On Hold"
	case "solved":
		return "Solved"
	case "closed":
		return "Closed"
	default:
		return s
	}
}

func formatPriority(p string) string {
	switch p {
	case "urgent":
		return "Urgent"
	case "high":
		return "High"
	case "normal":
		return "Normal"
	case "low":
		return "Low"
	default:
		return p
	}
}
