package chronicle

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getContactTimelineTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_contact_timeline",
			mcp.WithDescription("Get a chronological activity timeline for a contact, including emails, calls, notes, meetings, and deals."),
			mcp.WithString("contact_id",
				mcp.Required(),
				mcp.Description("CRM contact ID"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of timeline events to return (default: 30)"),
			),
		),
		Handler: p.handleGetContactTimeline,
	}
}

func (p *Provider) handleGetContactTimeline(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	contactID, ok := req.GetArguments()["contact_id"].(string)
	if !ok || contactID == "" {
		return mcp.NewToolResultError("contact_id is required"), nil
	}

	limit := 30
	if l, ok := req.GetArguments()["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	events, err := p.client.GetContactTimeline(ctx, contactID, limit)
	if err != nil {
		p.logger.Error("failed to get contact timeline", "contact_id", contactID, "error", err)
		return mcp.NewToolResultError("Failed to retrieve contact timeline. Please check the contact ID and try again."), nil
	}

	var sb strings.Builder
	sb.WriteString("## Contact Timeline\n\n")

	if len(events) == 0 {
		sb.WriteString("_No timeline events found._\n")
		return mcp.NewToolResultText(sb.String()), nil
	}

	sb.WriteString(fmt.Sprintf("Showing %d events (most recent first):\n\n", len(events)))
	sb.WriteString("| Date | Type | Title | Details |\n")
	sb.WriteString("| ---- | ---- | ----- | ------- |\n")

	for _, e := range events {
		details := e.Description
		if details == "" {
			details = "-"
		}
		// Truncate details for table format.
		if len(details) > 80 {
			details = details[:80] + "..."
		}
		// Escape pipe characters in table cells.
		details = strings.ReplaceAll(details, "|", "\\|")
		title := e.Title
		if title == "" {
			title = "-"
		}
		title = strings.ReplaceAll(title, "|", "\\|")

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			e.CreatedAt.Format("2006-01-02 15:04"),
			formatEventType(e.Type),
			title,
			details,
		))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func formatEventType(t string) string {
	switch t {
	case "email":
		return "Email"
	case "call":
		return "Call"
	case "note":
		return "Note"
	case "meeting":
		return "Meeting"
	case "deal":
		return "Deal"
	default:
		return t
	}
}
