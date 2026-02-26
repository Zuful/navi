package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getRevenueMetricsTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_revenue_metrics",
			mcp.WithDescription("Get MRR history, payment success rate, and outstanding invoices for a customer."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Stripe customer ID (e.g. cus_xxx)"),
			),
			mcp.WithNumber("months",
				mcp.Description("Number of months of history to retrieve (default: 6)"),
			),
		),
		Handler: p.handleGetRevenueMetrics,
	}
}

func (p *Provider) handleGetRevenueMetrics(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	months := 6
	if m, ok := req.GetArguments()["months"].(float64); ok && m > 0 {
		months = int(m)
	}

	metrics, err := p.client.GetRevenueMetrics(ctx, customerID, months)
	if err != nil {
		p.logger.Error("failed to get revenue metrics", "customer_id", customerID, "error", err)
		return mcp.NewToolResultError("Failed to retrieve revenue metrics. Please check the customer ID and try again."), nil
	}

	var sb strings.Builder
	sb.WriteString("## Revenue Metrics\n\n")
	sb.WriteString(fmt.Sprintf("**Customer:** %s (`%s`)\n\n", metrics.CustomerName, metrics.CustomerID))
	sb.WriteString(fmt.Sprintf("**Current MRR:** %s\n\n", formatCurrency(metrics.CurrentMRR, metrics.Currency)))
	sb.WriteString(fmt.Sprintf("**Payment Success Rate:** %.1f%%\n\n", metrics.PaymentSuccessRate))

	// MRR History.
	sb.WriteString("### MRR History\n\n")
	sb.WriteString("| Month | MRR |\n")
	sb.WriteString("| ----- | --- |\n")
	for _, dp := range metrics.MRRHistory {
		sb.WriteString(fmt.Sprintf("| %s | %s |\n", dp.Month, formatCurrency(dp.MRR, metrics.Currency)))
	}
	sb.WriteString("\n")

	// Outstanding Invoices.
	sb.WriteString("### Outstanding Invoices\n\n")
	if len(metrics.OutstandingInvoices) == 0 {
		sb.WriteString("_No outstanding invoices._\n")
	} else {
		sb.WriteString("| Invoice | Amount | Status | Due Date |\n")
		sb.WriteString("| ------- | ------ | ------ | -------- |\n")
		for _, inv := range metrics.OutstandingInvoices {
			due := "-"
			if inv.DueDate != nil {
				due = inv.DueDate.Format("2006-01-02")
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n",
				inv.ID, formatCurrency(inv.Amount, inv.Currency), inv.Status, due))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}
