package scout

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Zuful/navi/internal/provider"
)

func (p *Provider) getChurnRiskTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Tool: mcp.NewTool("get_churn_risk",
			mcp.WithDescription("Assess churn risk for a specific customer with contributing factors from billing and communication signals."),
			mcp.WithString("customer_id",
				mcp.Required(),
				mcp.Description("Customer ID (used across billing and CRM systems)"),
			),
		),
		Handler: p.handleGetChurnRisk,
	}
}

type churnFactor struct {
	signal string
	impact string // high, medium, low
	detail string
}

func (p *Provider) handleGetChurnRisk(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	customerID, ok := req.GetArguments()["customer_id"].(string)
	if !ok || customerID == "" {
		return mcp.NewToolResultError("customer_id is required"), nil
	}

	var factors []churnFactor
	riskScore := 0.0
	signalCount := 0

	// Billing factors.
	if p.billing != nil {
		sub, err := p.billing.GetSubscriptionStatus(ctx, customerID)
		if err != nil {
			p.logger.Warn("billing signal unavailable for churn assessment", "error", err)
		} else {
			signalCount++
			switch sub.Status {
			case "past_due":
				factors = append(factors, churnFactor{
					signal: "Payment Past Due",
					impact: "high",
					detail: "Customer has overdue payments",
				})
				riskScore += 80
			case "unpaid":
				factors = append(factors, churnFactor{
					signal: "Unpaid Subscription",
					impact: "high",
					detail: "Subscription is unpaid",
				})
				riskScore += 90
			case "canceled":
				factors = append(factors, churnFactor{
					signal: "Subscription Canceled",
					impact: "high",
					detail: "Customer has already canceled",
				})
				riskScore += 100
			case "trialing":
				factors = append(factors, churnFactor{
					signal: "In Trial",
					impact: "medium",
					detail: "Customer has not converted yet",
				})
				riskScore += 50
			case "active":
				riskScore += 10 // low risk
			}

			if sub.CancelAt != nil {
				factors = append(factors, churnFactor{
					signal: "Pending Cancellation",
					impact: "high",
					detail: fmt.Sprintf("Scheduled to cancel on %s", sub.CancelAt.Format("2006-01-02")),
				})
				riskScore += 40
			}
		}

		metrics, err := p.billing.GetRevenueMetrics(ctx, customerID, 3)
		if err != nil {
			p.logger.Warn("revenue signal unavailable for churn assessment", "error", err)
		} else {
			signalCount++
			if metrics.PaymentSuccessRate < 90 {
				factors = append(factors, churnFactor{
					signal: "Low Payment Success",
					impact: "medium",
					detail: fmt.Sprintf("%.0f%% payment success rate", metrics.PaymentSuccessRate),
				})
				riskScore += 30
			}
			if len(metrics.OutstandingInvoices) > 0 {
				factors = append(factors, churnFactor{
					signal: "Outstanding Invoices",
					impact: "medium",
					detail: fmt.Sprintf("%d unpaid invoices", len(metrics.OutstandingInvoices)),
				})
				riskScore += float64(len(metrics.OutstandingInvoices)) * 15
			}

			// Check for MRR decline.
			if len(metrics.MRRHistory) >= 2 {
				latest := metrics.MRRHistory[len(metrics.MRRHistory)-1].MRR
				prev := metrics.MRRHistory[len(metrics.MRRHistory)-2].MRR
				if prev > 0 && latest < prev {
					decline := float64(prev-latest) / float64(prev) * 100
					factors = append(factors, churnFactor{
						signal: "MRR Decline",
						impact: "medium",
						detail: fmt.Sprintf("%.1f%% decrease in MRR", decline),
					})
					riskScore += decline * 0.5
				}
			}
		}
	}

	// Support ticket factors.
	if p.support != nil {
		tickets, err := p.support.GetOpenTickets(ctx, customerID)
		if err != nil {
			p.logger.Warn("support signal unavailable for churn assessment", "error", err)
		} else {
			signalCount++
			if len(tickets) > 5 {
				factors = append(factors, churnFactor{
					signal: "High Ticket Volume",
					impact: "high",
					detail: fmt.Sprintf("%d open/pending support tickets", len(tickets)),
				})
				riskScore += 50
			} else if len(tickets) > 2 {
				factors = append(factors, churnFactor{
					signal: "Elevated Ticket Volume",
					impact: "medium",
					detail: fmt.Sprintf("%d open/pending support tickets", len(tickets)),
				})
				riskScore += 25
			}
		}

		satMetrics, err := p.support.GetSatisfactionScores(ctx, customerID)
		if err != nil {
			p.logger.Warn("satisfaction signal unavailable for churn assessment", "error", err)
		} else {
			if satMetrics.TotalRatings > 0 && satMetrics.AverageCSAT < 50 {
				factors = append(factors, churnFactor{
					signal: "Low CSAT Score",
					impact: "high",
					detail: fmt.Sprintf("%.0f%% average satisfaction (%d ratings)", satMetrics.AverageCSAT, satMetrics.TotalRatings),
				})
				riskScore += 40
			}
			if satMetrics.AverageResolutionHours > 48 {
				factors = append(factors, churnFactor{
					signal: "Slow Resolution Times",
					impact: "medium",
					detail: fmt.Sprintf("%.1f hour average resolution time", satMetrics.AverageResolutionHours),
				})
				riskScore += 20
			}
		}
	}

	// Usage factors.
	if p.usage != nil {
		trend, err := p.usage.GetUsageTrend(ctx, customerID, 3)
		if err != nil {
			p.logger.Warn("usage signal unavailable for churn assessment", "error", err)
		} else {
			signalCount++
			if trend.ChangePercent < -30 {
				factors = append(factors, churnFactor{
					signal: "Declining Usage",
					impact: "high",
					detail: fmt.Sprintf("%.1f%% decrease in product usage", -trend.ChangePercent),
				})
				riskScore += 60
			} else if trend.ChangePercent < -10 {
				factors = append(factors, churnFactor{
					signal: "Usage Slowdown",
					impact: "medium",
					detail: fmt.Sprintf("%.1f%% decrease in product usage", -trend.ChangePercent),
				})
				riskScore += 30
			}
		}

		summary, err := p.usage.GetUsageSummary(ctx, customerID, 30)
		if err != nil {
			p.logger.Warn("usage summary unavailable for churn assessment", "error", err)
		} else {
			if summary.DAU == 0 {
				factors = append(factors, churnFactor{
					signal: "No Recent Activity",
					impact: "high",
					detail: "No daily active users in the last 30 days",
				})
				riskScore += 50
			}
		}
	}

	// Communication factors.
	if p.comms != nil {
		comms, err := p.comms.GetRecentCommunications(ctx, customerID, 10)
		if err != nil {
			p.logger.Warn("comms signal unavailable for churn assessment", "error", err)
		} else {
			signalCount++
			if len(comms) == 0 {
				factors = append(factors, churnFactor{
					signal: "No Recent Communication",
					impact: "high",
					detail: "No communications found — customer may be disengaged",
				})
				riskScore += 60
			} else {
				// Check recency of last communication.
				latest := comms[0].CreatedAt
				daysSince := time.Since(latest).Hours() / 24
				if daysSince > 30 {
					factors = append(factors, churnFactor{
						signal: "Communication Gap",
						impact: "medium",
						detail: fmt.Sprintf("Last contact was %.0f days ago", daysSince),
					})
					riskScore += 30
				}
			}
		}
	}

	// Normalize score.
	var normalizedRisk float64
	if signalCount > 0 {
		normalizedRisk = riskScore / float64(signalCount)
		if normalizedRisk > 100 {
			normalizedRisk = 100
		}
	}

	// Build output.
	var sb strings.Builder
	sb.WriteString("## Churn Risk Assessment\n\n")
	sb.WriteString(fmt.Sprintf("**Customer ID:** `%s`\n\n", customerID))

	if signalCount == 0 {
		sb.WriteString("**Risk Level:** N/A — _No signals available_\n\n")
		sb.WriteString("_Configure billing and/or communications providers to enable churn prediction._\n")
		return mcp.NewToolResultText(sb.String()), nil
	}

	riskLevel := riskLabel(normalizedRisk)
	sb.WriteString(fmt.Sprintf("**Risk Score:** %.0f/100 — **%s**\n\n", normalizedRisk, riskLevel))

	if len(factors) == 0 {
		sb.WriteString("### Contributing Factors\n\n_No risk factors detected. Customer appears healthy._\n")
	} else {
		sb.WriteString("### Contributing Factors\n\n")
		sb.WriteString("| Factor | Impact | Details |\n")
		sb.WriteString("| ------ | ------ | ------- |\n")
		for _, f := range factors {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", f.signal, formatImpact(f.impact), f.detail))
		}
	}

	sb.WriteString("\n### Recommendations\n\n")
	for _, rec := range recommendations(normalizedRisk, factors) {
		sb.WriteString(fmt.Sprintf("- %s\n", rec))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func riskLabel(score float64) string {
	switch {
	case score >= 70:
		return "High Risk"
	case score >= 40:
		return "Medium Risk"
	case score >= 20:
		return "Low Risk"
	default:
		return "Minimal Risk"
	}
}

func formatImpact(impact string) string {
	switch impact {
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return impact
	}
}

func recommendations(score float64, factors []churnFactor) []string {
	var recs []string

	hasPaymentIssue := false
	hasCommGap := false
	hasCancellation := false
	hasSupportIssue := false
	hasUsageDecline := false

	for _, f := range factors {
		switch f.signal {
		case "Payment Past Due", "Unpaid Subscription", "Low Payment Success", "Outstanding Invoices":
			hasPaymentIssue = true
		case "No Recent Communication", "Communication Gap":
			hasCommGap = true
		case "Subscription Canceled", "Pending Cancellation":
			hasCancellation = true
		case "High Ticket Volume", "Elevated Ticket Volume", "Low CSAT Score", "Slow Resolution Times":
			hasSupportIssue = true
		case "Declining Usage", "Usage Slowdown", "No Recent Activity":
			hasUsageDecline = true
		}
	}

	if hasCancellation {
		recs = append(recs, "Schedule an immediate retention call to understand cancellation reasons")
		recs = append(recs, "Prepare a tailored win-back offer or discount")
	}

	if hasPaymentIssue {
		recs = append(recs, "Reach out about payment issues — offer payment plan if needed")
		recs = append(recs, "Verify payment method is up to date")
	}

	if hasSupportIssue {
		recs = append(recs, "Review open support tickets and prioritize resolution")
		recs = append(recs, "Escalate unresolved tickets to senior support or engineering")
	}

	if hasUsageDecline {
		recs = append(recs, "Review product usage patterns and identify adoption gaps")
		recs = append(recs, "Schedule a product training or onboarding refresher session")
	}

	if hasCommGap {
		recs = append(recs, "Schedule a check-in call or send a personalized email")
		recs = append(recs, "Share relevant product updates or success stories")
	}

	if score >= 70 && len(recs) == 0 {
		recs = append(recs, "Escalate to CS leadership for immediate attention")
	}

	if len(recs) == 0 {
		recs = append(recs, "Continue regular engagement cadence")
		recs = append(recs, "Monitor for any changes in usage or payment patterns")
	}

	return recs
}
