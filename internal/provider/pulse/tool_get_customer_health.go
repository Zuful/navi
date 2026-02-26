package pulse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getCustomerHealthTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_customer_health",
			mcp.WithDescription("Get a composite health score for a customer based on available signals (billing status, communication recency, payment history)."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Customer ID (used across billing and CRM systems)"),
			),
		),
		Handler: p.handleGetCustomerHealth,
	}
}

func (p *Provider) handleGetCustomerHealth(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	var sb strings.Builder
	sb.WriteString("## Customer Health Report\n\n")
	sb.WriteString(fmt.Sprintf("**Customer ID:** `%s`\n\n", customerID))

	totalScore := 0.0
	signalCount := 0

	sb.WriteString("### Signals\n\n")
	sb.WriteString("| Signal | Score | Details |\n")
	sb.WriteString("| ------ | ----- | ------- |\n")

	// Billing signals.
	if p.billing != nil {
		sub, err := p.billing.GetSubscriptionStatus(ctx, customerID)
		if err != nil {
			p.logger.Warn("billing signal unavailable", "error", err)
			sb.WriteString("| Subscription Status | - | _Unavailable_ |\n")
		} else {
			score := scoreBillingStatus(sub.Status)
			totalScore += score
			signalCount++
			sb.WriteString(fmt.Sprintf("| Subscription Status | %.0f/100 | %s |\n", score, sub.Status))
		}

		metrics, err := p.billing.GetRevenueMetrics(ctx, customerID, 3)
		if err != nil {
			p.logger.Warn("revenue signal unavailable", "error", err)
			sb.WriteString("| Payment History | - | _Unavailable_ |\n")
		} else {
			score := scorePaymentHistory(metrics.PaymentSuccessRate, len(metrics.OutstandingInvoices))
			totalScore += score
			signalCount++
			details := fmt.Sprintf("%.0f%% success rate, %d outstanding invoices",
				metrics.PaymentSuccessRate, len(metrics.OutstandingInvoices))
			sb.WriteString(fmt.Sprintf("| Payment History | %.0f/100 | %s |\n", score, details))
		}
	} else {
		sb.WriteString("| Billing | - | _Provider not configured_ |\n")
	}

	// Support ticket signals.
	if p.support != nil {
		tickets, err := p.support.GetOpenTickets(ctx, customerID)
		if err != nil {
			p.logger.Warn("support signal unavailable", "error", err)
			sb.WriteString("| Support Ticket Load | - | _Unavailable_ |\n")
		} else {
			score := scoreSupportTicketLoad(len(tickets))
			totalScore += score
			signalCount++
			details := fmt.Sprintf("%d open/pending tickets", len(tickets))
			sb.WriteString(fmt.Sprintf("| Support Ticket Load | %.0f/100 | %s |\n", score, details))
		}
	} else {
		sb.WriteString("| Support Tickets | - | _Provider not configured_ |\n")
	}

	// Communications signals.
	if p.comms != nil {
		comms, err := p.comms.GetRecentCommunications(ctx, customerID, 10)
		if err != nil {
			p.logger.Warn("comms signal unavailable", "error", err)
			sb.WriteString("| Communication Recency | - | _Unavailable_ |\n")
		} else {
			var latestTime time.Time
			if len(comms) > 0 {
				latestTime = comms[0].CreatedAt
			}
			score, details := scoreCommsRecencyFromCount(len(comms), latestTime)
			totalScore += score
			signalCount++
			sb.WriteString(fmt.Sprintf("| Communication Recency | %.0f/100 | %s |\n", score, details))
		}
	} else {
		sb.WriteString("| Communications | - | _Provider not configured_ |\n")
	}

	sb.WriteString("\n")

	// Overall health score.
	if signalCount > 0 {
		overallScore := totalScore / float64(signalCount)
		healthLabel := healthLabel(overallScore)
		sb.WriteString(fmt.Sprintf("### Overall Health: **%.0f/100** — %s\n\n", overallScore, healthLabel))
	} else {
		sb.WriteString("### Overall Health: **N/A** — _No signals available_\n\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func scoreBillingStatus(status string) float64 {
	switch status {
	case "active":
		return 100
	case "trialing":
		return 80
	case "past_due":
		return 30
	case "unpaid":
		return 10
	case "canceled":
		return 0
	case "no_subscription":
		return 0
	default:
		return 50
	}
}

func scorePaymentHistory(successRate float64, outstandingCount int) float64 {
	score := successRate // 0-100
	// Penalize for outstanding invoices.
	penalty := float64(outstandingCount) * 15
	score -= penalty
	if score < 0 {
		score = 0
	}
	return score
}

func scoreCommsRecencyFromCount(count int, latest time.Time) (float64, string) {
	if count == 0 {
		return 20, "No recent communications"
	}

	if latest.IsZero() {
		// Can't determine recency without timestamps.
		if count > 5 {
			return 80, fmt.Sprintf("%d recent communications", count)
		}
		return 60, fmt.Sprintf("%d recent communications", count)
	}

	daysSince := time.Since(latest).Hours() / 24
	var score float64
	var details string

	switch {
	case daysSince <= 7:
		score = 100
		details = fmt.Sprintf("Last contact %.0f days ago (%d total)", daysSince, count)
	case daysSince <= 14:
		score = 80
		details = fmt.Sprintf("Last contact %.0f days ago (%d total)", daysSince, count)
	case daysSince <= 30:
		score = 60
		details = fmt.Sprintf("Last contact %.0f days ago (%d total)", daysSince, count)
	case daysSince <= 60:
		score = 40
		details = fmt.Sprintf("Last contact %.0f days ago (%d total)", daysSince, count)
	default:
		score = 20
		details = fmt.Sprintf("Last contact %.0f days ago (%d total)", daysSince, count)
	}

	return score, details
}

func scoreSupportTicketLoad(openCount int) float64 {
	switch {
	case openCount == 0:
		return 100
	case openCount <= 2:
		return 80
	case openCount <= 5:
		return 60
	case openCount <= 10:
		return 40
	default:
		return 20
	}
}

func healthLabel(score float64) string {
	switch {
	case score >= 80:
		return "Healthy"
	case score >= 60:
		return "Needs Attention"
	case score >= 40:
		return "At Risk"
	default:
		return "Critical"
	}
}
