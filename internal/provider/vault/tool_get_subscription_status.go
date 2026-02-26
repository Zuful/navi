package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getSubscriptionStatusTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_subscription_status",
			mcp.WithDescription("Get subscription plan, status, MRR, and trial info for a customer."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Stripe customer ID (e.g. cus_xxx)"),
			),
		),
		Handler: p.handleGetSubscriptionStatus,
	}
}

func (p *Provider) handleGetSubscriptionStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	sub, err := p.client.GetSubscriptionStatus(ctx, customerID)
	if err != nil {
		p.logger.Error("failed to get subscription status", "customer_id", customerID, "error", err)
		return mcp.NewToolResultError("Failed to retrieve subscription status. Please check the customer ID and try again."), nil
	}

	var sb strings.Builder
	sb.WriteString("## Subscription Status\n\n")

	sb.WriteString(fmt.Sprintf("| Field | Value |\n"))
	sb.WriteString("| ----- | ----- |\n")
	sb.WriteString(fmt.Sprintf("| Customer | %s |\n", sub.CustomerName))
	sb.WriteString(fmt.Sprintf("| Customer ID | `%s` |\n", sub.CustomerID))
	sb.WriteString(fmt.Sprintf("| Plan | %s |\n", sub.Plan))
	sb.WriteString(fmt.Sprintf("| Status | %s |\n", formatStatus(sub.Status)))
	sb.WriteString(fmt.Sprintf("| MRR | %s |\n", formatCurrency(sub.MRR, sub.Currency)))
	sb.WriteString(fmt.Sprintf("| Period End | %s |\n", sub.CurrentPeriodEnd.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("| Created | %s |\n", sub.CreatedAt.Format("2006-01-02")))

	if sub.TrialEnd != nil {
		sb.WriteString(fmt.Sprintf("| Trial End | %s |\n", sub.TrialEnd.Format("2006-01-02")))
	}
	if sub.CancelAt != nil {
		sb.WriteString(fmt.Sprintf("| Cancel At | %s |\n", sub.CancelAt.Format("2006-01-02")))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func formatCurrency(cents int64, currency string) string {
	dollars := float64(cents) / 100
	symbol := "$"
	switch strings.ToLower(currency) {
	case "eur":
		symbol = "\u20ac"
	case "gbp":
		symbol = "\u00a3"
	}
	return fmt.Sprintf("%s%.2f %s/mo", symbol, dollars, strings.ToUpper(currency))
}

func formatStatus(status string) string {
	switch status {
	case "active":
		return "Active"
	case "trialing":
		return "Trialing"
	case "past_due":
		return "Past Due"
	case "canceled":
		return "Canceled"
	case "unpaid":
		return "Unpaid"
	case "no_subscription":
		return "No Subscription"
	default:
		return status
	}
}
