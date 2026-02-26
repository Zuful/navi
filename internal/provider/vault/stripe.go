package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Zuful/navi/internal/httpclient"
)

// StripeClient implements BillingClient using the Stripe API.
type StripeClient struct {
	apiKey string
	http   *httpclient.Client
}

// NewStripeClient creates a new Stripe billing client.
func NewStripeClient(apiKey string, httpClient *httpclient.Client) *StripeClient {
	return &StripeClient{apiKey: apiKey, http: httpClient}
}

// stripeRequest builds and executes an authenticated Stripe API request.
func (s *StripeClient) stripeRequest(ctx context.Context, path string) ([]byte, error) {
	url := "https://api.stripe.com/v1" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	return s.http.Do(ctx, req)
}

// --- Stripe response types ---

type stripeList struct {
	Data    json.RawMessage `json:"data"`
	HasMore bool            `json:"has_more"`
}

type stripeSubscription struct {
	ID               string              `json:"id"`
	Status           string              `json:"status"`
	Created          int64               `json:"created"`
	CurrentPeriodEnd int64               `json:"current_period_end"`
	TrialEnd         *int64              `json:"trial_end"`
	CancelAt         *int64              `json:"cancel_at"`
	Customer         string              `json:"customer"`
	Items            stripeSubItems      `json:"items"`
	Plan             *stripeSubscriptionPlan `json:"plan"`
}

type stripeSubItems struct {
	Data []stripeSubItem `json:"data"`
}

type stripeSubItem struct {
	Price stripePrice `json:"price"`
}

type stripePrice struct {
	UnitAmount int64    `json:"unit_amount"`
	Currency   string   `json:"currency"`
	Recurring  *stripeRecurring `json:"recurring"`
	Product    string   `json:"product"`
	Nickname   string   `json:"nickname"`
}

type stripeRecurring struct {
	Interval      string `json:"interval"`
	IntervalCount int    `json:"interval_count"`
}

type stripeSubscriptionPlan struct {
	Nickname string `json:"nickname"`
	Product  string `json:"product"`
}

type stripeCustomer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type stripeInvoice struct {
	ID         string `json:"id"`
	AmountDue  int64  `json:"amount_due"`
	Currency   string `json:"currency"`
	Status     string `json:"status"`
	DueDate    *int64 `json:"due_date"`
	Created    int64  `json:"created"`
	Paid       bool   `json:"paid"`
}

type stripeCharge struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (s *StripeClient) getCustomer(ctx context.Context, customerID string) (*stripeCustomer, error) {
	data, err := s.stripeRequest(ctx, "/customers/"+customerID)
	if err != nil {
		return nil, err
	}
	var cust stripeCustomer
	if err := json.Unmarshal(data, &cust); err != nil {
		return nil, fmt.Errorf("parse customer: %w", err)
	}
	return &cust, nil
}

// GetSubscriptionStatus returns the primary subscription for a Stripe customer.
func (s *StripeClient) GetSubscriptionStatus(ctx context.Context, customerID string) (*Subscription, error) {
	cust, err := s.getCustomer(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("get customer: %w", err)
	}

	data, err := s.stripeRequest(ctx, "/subscriptions?customer="+customerID+"&limit=1&status=all")
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}

	var list stripeList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse subscription list: %w", err)
	}

	var subs []stripeSubscription
	if err := json.Unmarshal(list.Data, &subs); err != nil {
		return nil, fmt.Errorf("parse subscriptions: %w", err)
	}

	if len(subs) == 0 {
		return &Subscription{
			CustomerID:   customerID,
			CustomerName: cust.Name,
			Status:       "no_subscription",
		}, nil
	}

	sub := subs[0]

	// Calculate MRR from subscription items.
	var mrr int64
	currency := "usd"
	planName := "Unknown"

	if len(sub.Items.Data) > 0 {
		item := sub.Items.Data[0]
		currency = item.Price.Currency
		mrr = toMRR(item.Price.UnitAmount, item.Price.Recurring)
		if item.Price.Nickname != "" {
			planName = item.Price.Nickname
		}
	}

	if sub.Plan != nil && sub.Plan.Nickname != "" {
		planName = sub.Plan.Nickname
	}

	result := &Subscription{
		CustomerID:       customerID,
		CustomerName:     cust.Name,
		Plan:             planName,
		Status:           sub.Status,
		MRR:              mrr,
		Currency:         currency,
		CurrentPeriodEnd: time.Unix(sub.CurrentPeriodEnd, 0),
		CreatedAt:        time.Unix(sub.Created, 0),
	}

	if sub.TrialEnd != nil {
		t := time.Unix(*sub.TrialEnd, 0)
		result.TrialEnd = &t
	}
	if sub.CancelAt != nil {
		t := time.Unix(*sub.CancelAt, 0)
		result.CancelAt = &t
	}

	return result, nil
}

// GetRevenueMetrics returns revenue metrics for a customer.
func (s *StripeClient) GetRevenueMetrics(ctx context.Context, customerID string, months int) (*RevenueMetrics, error) {
	cust, err := s.getCustomer(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("get customer: %w", err)
	}

	// Get invoices for MRR history.
	since := time.Now().AddDate(0, -months, 0).Unix()
	path := fmt.Sprintf("/invoices?customer=%s&created[gte]=%d&limit=100&status=paid", customerID, since)
	data, err := s.stripeRequest(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}

	var list stripeList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse invoice list: %w", err)
	}

	var invoices []stripeInvoice
	if err := json.Unmarshal(list.Data, &invoices); err != nil {
		return nil, fmt.Errorf("parse invoices: %w", err)
	}

	// Build MRR history by month.
	mrrByMonth := make(map[string]int64)
	for _, inv := range invoices {
		month := time.Unix(inv.Created, 0).Format("2006-01")
		mrrByMonth[month] += inv.AmountDue
	}

	var mrrHistory []MRRDataPoint
	for i := months - 1; i >= 0; i-- {
		month := time.Now().AddDate(0, -i, 0).Format("2006-01")
		mrrHistory = append(mrrHistory, MRRDataPoint{
			Month: month,
			MRR:   mrrByMonth[month],
		})
	}

	// Get payment success rate from charges.
	chargePath := fmt.Sprintf("/charges?customer=%s&created[gte]=%d&limit=100", customerID, since)
	chargeData, err := s.stripeRequest(ctx, chargePath)
	if err != nil {
		return nil, fmt.Errorf("list charges: %w", err)
	}

	var chargeList stripeList
	if err := json.Unmarshal(chargeData, &chargeList); err != nil {
		return nil, fmt.Errorf("parse charge list: %w", err)
	}

	var charges []stripeCharge
	if err := json.Unmarshal(chargeList.Data, &charges); err != nil {
		return nil, fmt.Errorf("parse charges: %w", err)
	}

	var successRate float64
	if len(charges) > 0 {
		succeeded := 0
		for _, c := range charges {
			if c.Status == "succeeded" {
				succeeded++
			}
		}
		successRate = float64(succeeded) / float64(len(charges)) * 100
	}

	// Get outstanding invoices.
	openPath := fmt.Sprintf("/invoices?customer=%s&status=open&limit=10", customerID)
	openData, err := s.stripeRequest(ctx, openPath)
	if err != nil {
		return nil, fmt.Errorf("list open invoices: %w", err)
	}

	var openList stripeList
	if err := json.Unmarshal(openData, &openList); err != nil {
		return nil, fmt.Errorf("parse open invoice list: %w", err)
	}

	var openInvoices []stripeInvoice
	if err := json.Unmarshal(openList.Data, &openInvoices); err != nil {
		return nil, fmt.Errorf("parse open invoices: %w", err)
	}

	var outstanding []Invoice
	for _, inv := range openInvoices {
		oi := Invoice{
			ID:        inv.ID,
			Amount:    inv.AmountDue,
			Currency:  inv.Currency,
			Status:    inv.Status,
			CreatedAt: time.Unix(inv.Created, 0),
		}
		if inv.DueDate != nil {
			t := time.Unix(*inv.DueDate, 0)
			oi.DueDate = &t
		}
		outstanding = append(outstanding, oi)
	}

	// Get current MRR from active subscription.
	var currentMRR int64
	currency := "usd"
	subData, err := s.stripeRequest(ctx, "/subscriptions?customer="+customerID+"&limit=1")
	if err == nil {
		var subList stripeList
		if json.Unmarshal(subData, &subList) == nil {
			var subs []stripeSubscription
			if json.Unmarshal(subList.Data, &subs) == nil && len(subs) > 0 {
				if len(subs[0].Items.Data) > 0 {
					item := subs[0].Items.Data[0]
					currentMRR = toMRR(item.Price.UnitAmount, item.Price.Recurring)
					currency = item.Price.Currency
				}
			}
		}
	}

	return &RevenueMetrics{
		CustomerID:          customerID,
		CustomerName:        cust.Name,
		CurrentMRR:          currentMRR,
		Currency:            currency,
		MRRHistory:          mrrHistory,
		PaymentSuccessRate:  successRate,
		OutstandingInvoices: outstanding,
	}, nil
}

// GetRenewalCalendar returns upcoming renewals within daysAhead.
func (s *StripeClient) GetRenewalCalendar(ctx context.Context, daysAhead int) ([]Renewal, error) {
	now := time.Now()
	cutoff := now.AddDate(0, 0, daysAhead)

	// Get active and trialing subscriptions.
	path := fmt.Sprintf("/subscriptions?limit=100&current_period_end[lte]=%d&current_period_end[gte]=%d",
		cutoff.Unix(), now.Unix())
	data, err := s.stripeRequest(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}

	var list stripeList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse list: %w", err)
	}

	var subs []stripeSubscription
	if err := json.Unmarshal(list.Data, &subs); err != nil {
		return nil, fmt.Errorf("parse subscriptions: %w", err)
	}

	// Also check trialing subscriptions with trial_end in range.
	trialPath := fmt.Sprintf("/subscriptions?status=trialing&limit=100")
	trialData, err := s.stripeRequest(ctx, trialPath)
	if err == nil {
		var trialList stripeList
		if json.Unmarshal(trialData, &trialList) == nil {
			var trialSubs []stripeSubscription
			if json.Unmarshal(trialList.Data, &trialSubs) == nil {
				subs = append(subs, trialSubs...)
			}
		}
	}

	// Deduplicate by subscription ID.
	seen := make(map[string]bool)
	var renewals []Renewal

	for _, sub := range subs {
		if seen[sub.ID] {
			continue
		}
		seen[sub.ID] = true

		// Determine renewal status.
		status := "renewing"
		renewalDate := time.Unix(sub.CurrentPeriodEnd, 0)

		if sub.CancelAt != nil {
			status = "expiring"
		}
		if sub.TrialEnd != nil && sub.Status == "trialing" {
			trialEnd := time.Unix(*sub.TrialEnd, 0)
			if trialEnd.Before(cutoff) {
				status = "trial_ending"
				renewalDate = trialEnd
			}
		}

		var mrr int64
		currency := "usd"
		planName := "Unknown"

		if len(sub.Items.Data) > 0 {
			item := sub.Items.Data[0]
			mrr = toMRR(item.Price.UnitAmount, item.Price.Recurring)
			currency = item.Price.Currency
			if item.Price.Nickname != "" {
				planName = item.Price.Nickname
			}
		}

		// Fetch customer name.
		custName := sub.Customer
		cust, err := s.getCustomer(ctx, sub.Customer)
		if err == nil && cust.Name != "" {
			custName = cust.Name
		}

		renewals = append(renewals, Renewal{
			CustomerID:   sub.Customer,
			CustomerName: custName,
			Plan:         planName,
			MRR:          mrr,
			Currency:     currency,
			RenewalDate:  renewalDate,
			Status:       status,
		})
	}

	return renewals, nil
}

// toMRR normalizes a price to monthly recurring revenue.
func toMRR(unitAmount int64, recurring *stripeRecurring) int64 {
	if recurring == nil {
		return unitAmount
	}
	switch recurring.Interval {
	case "year":
		return unitAmount / (12 * int64(recurring.IntervalCount))
	case "month":
		return unitAmount / int64(recurring.IntervalCount)
	case "week":
		return unitAmount * 4 / int64(recurring.IntervalCount) // approximate
	case "day":
		return unitAmount * 30 / int64(recurring.IntervalCount) // approximate
	default:
		return unitAmount
	}
}
