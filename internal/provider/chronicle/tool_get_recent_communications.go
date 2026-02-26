package chronicle

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getRecentCommunicationsTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_recent_communications",
			mcp.WithDescription("Get recent emails, notes, and calls for a customer from the CRM."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("CRM customer/company ID"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of communications to return (default: 20)"),
			),
		),
		Handler: p.handleGetRecentCommunications,
	}
}

func (p *Provider) handleGetRecentCommunications(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	limit := 20
	if l, ok := req.GetArguments()["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	comms, err := p.client.GetRecentCommunications(ctx, customerID, limit)
	if err != nil {
		p.logger.Error("failed to get recent communications", "customer_id", customerID, "error", err)
		return mcp.NewToolResultError("Failed to retrieve recent communications. Please check the customer ID and try again."), nil
	}

	var sb strings.Builder
	sb.WriteString("## Recent Communications\n\n")

	if len(comms) == 0 {
		sb.WriteString("_No recent communications found._\n")
		return mcp.NewToolResultText(sb.String()), nil
	}

	sb.WriteString(fmt.Sprintf("Showing %d most recent communications:\n\n", len(comms)))

	for i, c := range comms {
		sb.WriteString(fmt.Sprintf("### %d. %s — %s\n\n", i+1, formatCommType(c.Type), c.CreatedAt.Format("2006-01-02 15:04")))

		if c.Subject != "" {
			sb.WriteString(fmt.Sprintf("**Subject:** %s\n\n", c.Subject))
		}
		if c.Direction != "" {
			sb.WriteString(fmt.Sprintf("**Direction:** %s\n\n", c.Direction))
		}
		if c.ContactName != "" {
			sb.WriteString(fmt.Sprintf("**Contact:** %s", c.ContactName))
			if c.ContactEmail != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", c.ContactEmail))
			}
			sb.WriteString("\n\n")
		}
		if c.Body != "" {
			sb.WriteString(fmt.Sprintf("> %s\n\n", strings.ReplaceAll(c.Body, "\n", "\n> ")))
		}

		sb.WriteString("---\n\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func formatCommType(t string) string {
	switch t {
	case "email":
		return "Email"
	case "call":
		return "Call"
	case "note":
		return "Note"
	case "meeting":
		return "Meeting"
	case "task":
		return "Task"
	default:
		return t
	}
}
