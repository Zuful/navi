package beacon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Zuful/navi/internal/httpclient"
)

// ZendeskClient implements SupportClient using the Zendesk API v2.
type ZendeskClient struct {
	apiKey    string
	subdomain string
	http      *httpclient.Client
}

// NewZendeskClient creates a new Zendesk support client.
func NewZendeskClient(apiKey, subdomain string, httpClient *httpclient.Client) *ZendeskClient {
	return &ZendeskClient{apiKey: apiKey, subdomain: subdomain, http: httpClient}
}

// zendeskRequest builds and executes an authenticated Zendesk API request.
func (z *ZendeskClient) zendeskRequest(ctx context.Context, path string) ([]byte, error) {
	u := fmt.Sprintf("https://%s.zendesk.com/api/v2%s", z.subdomain, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+z.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return z.http.Do(ctx, req)
}

// --- Zendesk response types ---

type zendeskSearchResults struct {
	Results []json.RawMessage `json:"results"`
	Count   int               `json:"count"`
}

type zendeskTicket struct {
	ID          int64    `json:"id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Via         zendeskVia `json:"via"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	AssigneeID  *int64   `json:"assignee_id"`
	RequesterID int64    `json:"requester_id"`
}

type zendeskVia struct {
	Channel string `json:"channel"`
}

type zendeskUser struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type zendeskUserResponse struct {
	User zendeskUser `json:"user"`
}

type zendeskSatisfactionRating struct {
	ID        int64  `json:"id"`
	Score     string `json:"score"` // good, bad, offered, unoffered
	CreatedAt string `json:"created_at"`
}

type zendeskSatisfactionResults struct {
	SatisfactionRatings []zendeskSatisfactionRating `json:"satisfaction_ratings"`
}

func (z *ZendeskClient) getUser(ctx context.Context, userID int64) (*zendeskUser, error) {
	data, err := z.zendeskRequest(ctx, fmt.Sprintf("/users/%d.json", userID))
	if err != nil {
		return nil, err
	}
	var resp zendeskUserResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}
	return &resp.User, nil
}

func (z *ZendeskClient) searchTickets(ctx context.Context, query string) ([]zendeskTicket, error) {
	path := "/search.json?query=" + url.QueryEscape(query)
	data, err := z.zendeskRequest(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("search tickets: %w", err)
	}

	var results zendeskSearchResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse search results: %w", err)
	}

	var tickets []zendeskTicket
	for _, raw := range results.Results {
		var t zendeskTicket
		if json.Unmarshal(raw, &t) == nil {
			tickets = append(tickets, t)
		}
	}
	return tickets, nil
}

func (z *ZendeskClient) toTicket(ctx context.Context, zt zendeskTicket) Ticket {
	t := Ticket{
		ID:          fmt.Sprintf("%d", zt.ID),
		Subject:     zt.Subject,
		Description: truncateDescription(zt.Description, 500),
		Status:      zt.Status,
		Priority:    zt.Priority,
		Channel:     zt.Via.Channel,
		CreatedAt:   parseZendeskTime(zt.CreatedAt),
		UpdatedAt:   parseZendeskTime(zt.UpdatedAt),
	}

	// Fetch requester info.
	if user, err := z.getUser(ctx, zt.RequesterID); err == nil {
		t.CustomerID = fmt.Sprintf("%d", user.ID)
		t.RequesterName = user.Name
		t.RequesterEmail = user.Email
	}

	// Fetch assignee info.
	if zt.AssigneeID != nil {
		if user, err := z.getUser(ctx, *zt.AssigneeID); err == nil {
			t.AssignedTo = user.Name
		}
	}

	// Determine resolved time from status.
	if zt.Status == "solved" || zt.Status == "closed" {
		resolved := parseZendeskTime(zt.UpdatedAt)
		t.ResolvedAt = &resolved
	}

	return t
}

// GetOpenTickets returns open and pending tickets for a customer.
func (z *ZendeskClient) GetOpenTickets(ctx context.Context, customerID string) ([]Ticket, error) {
	query := fmt.Sprintf("type:ticket requester_id:%s status<solved", customerID)
	zTickets, err := z.searchTickets(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get open tickets: %w", err)
	}

	var tickets []Ticket
	for _, zt := range zTickets {
		tickets = append(tickets, z.toTicket(ctx, zt))
	}
	return tickets, nil
}

// GetTicketHistory returns ticket history for a customer (all statuses).
func (z *ZendeskClient) GetTicketHistory(ctx context.Context, customerID string, limit int) ([]Ticket, error) {
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf("type:ticket requester_id:%s", customerID)
	zTickets, err := z.searchTickets(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get ticket history: %w", err)
	}

	var tickets []Ticket
	for _, zt := range zTickets {
		tickets = append(tickets, z.toTicket(ctx, zt))
	}

	if len(tickets) > limit {
		tickets = tickets[:limit]
	}
	return tickets, nil
}

// GetSatisfactionScores returns CSAT and resolution metrics for a customer.
func (z *ZendeskClient) GetSatisfactionScores(ctx context.Context, customerID string) (*SatisfactionMetrics, error) {
	// Get all tickets for the customer to compute metrics.
	query := fmt.Sprintf("type:ticket requester_id:%s", customerID)
	zTickets, err := z.searchTickets(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get tickets for satisfaction: %w", err)
	}

	metrics := &SatisfactionMetrics{
		CustomerID: customerID,
	}

	var totalResolutionHours float64
	resolvedCount := 0

	for _, zt := range zTickets {
		switch zt.Status {
		case "open", "hold":
			metrics.OpenCount++
		case "pending":
			metrics.PendingCount++
		case "solved", "closed":
			created := parseZendeskTime(zt.CreatedAt)
			resolved := parseZendeskTime(zt.UpdatedAt)
			if !created.IsZero() && !resolved.IsZero() {
				hours := resolved.Sub(created).Hours()
				totalResolutionHours += hours
				resolvedCount++
			}
		}
	}

	if resolvedCount > 0 {
		metrics.AverageResolutionHours = totalResolutionHours / float64(resolvedCount)
	}

	// Fetch satisfaction ratings for each resolved ticket.
	var totalCSAT float64
	ratingCount := 0

	for _, zt := range zTickets {
		if zt.Status != "solved" && zt.Status != "closed" {
			continue
		}
		data, err := z.zendeskRequest(ctx, fmt.Sprintf("/tickets/%d/satisfaction_ratings.json", zt.ID))
		if err != nil {
			continue
		}
		var satResults zendeskSatisfactionResults
		if json.Unmarshal(data, &satResults) != nil {
			continue
		}
		for _, r := range satResults.SatisfactionRatings {
			switch r.Score {
			case "good":
				totalCSAT += 100
				ratingCount++
			case "bad":
				totalCSAT += 0
				ratingCount++
			}
		}
	}

	if ratingCount > 0 {
		metrics.AverageCSAT = totalCSAT / float64(ratingCount)
	}
	metrics.TotalRatings = ratingCount

	// Estimate first response hours (use 1/3 of resolution time as proxy).
	if metrics.AverageResolutionHours > 0 {
		metrics.FirstResponseHours = metrics.AverageResolutionHours / 3
	}

	return metrics, nil
}

func parseZendeskTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func truncateDescription(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
