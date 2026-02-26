package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Zuful/navi/internal/httpclient"
)

// MixpanelClient implements UsageClient using the Mixpanel API.
type MixpanelClient struct {
	apiKey    string
	projectID string
	http      *httpclient.Client
}

// NewMixpanelClient creates a new Mixpanel usage client.
func NewMixpanelClient(apiKey, projectID string, httpClient *httpclient.Client) *MixpanelClient {
	return &MixpanelClient{apiKey: apiKey, projectID: projectID, http: httpClient}
}

// mixpanelRequest builds and executes an authenticated Mixpanel API request.
func (m *MixpanelClient) mixpanelRequest(ctx context.Context, path string) ([]byte, error) {
	u := fmt.Sprintf("https://mixpanel.com/api/2.0%s", path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	q := req.URL.Query()
	q.Set("project_id", m.projectID)
	req.URL.RawQuery = q.Encode()

	return m.http.Do(ctx, req)
}

// --- Mixpanel response types ---

type mixpanelEngagementResponse struct {
	Results struct {
		DAU         int `json:"dau"`
		WAU         int `json:"wau"`
		MAU         int `json:"mau"`
		TotalEvents int `json:"total_events"`
	} `json:"results"`
}

type mixpanelTopEventsResponse struct {
	Events []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"events"`
}

type mixpanelFeatureResponse struct {
	Features []struct {
		Name        string `json:"name"`
		EventCount  int    `json:"event_count"`
		UniqueUsers int    `json:"unique_users"`
		LastUsed    string `json:"last_used"`
	} `json:"features"`
}

type mixpanelTrendResponse struct {
	Direction     string  `json:"direction"`
	ChangePercent float64 `json:"change_percent"`
	DataPoints    []struct {
		Period      string `json:"period"`
		ActiveUsers int    `json:"active_users"`
		EventCount  int    `json:"event_count"`
	} `json:"data_points"`
}

// GetUsageSummary returns engagement metrics for a customer over the given number of days.
func (m *MixpanelClient) GetUsageSummary(ctx context.Context, customerID string, days int) (*UsageSummary, error) {
	path := fmt.Sprintf("/engage?customer_id=%s&days=%d", customerID, days)
	data, err := m.mixpanelRequest(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get usage summary: %w", err)
	}

	var engResp mixpanelEngagementResponse
	if err := json.Unmarshal(data, &engResp); err != nil {
		return nil, fmt.Errorf("parse engagement response: %w", err)
	}

	// Fetch top events.
	evPath := fmt.Sprintf("/events/top?customer_id=%s&days=%d&limit=10", customerID, days)
	evData, err := m.mixpanelRequest(ctx, evPath)
	if err != nil {
		return nil, fmt.Errorf("get top events: %w", err)
	}

	var topResp mixpanelTopEventsResponse
	if err := json.Unmarshal(evData, &topResp); err != nil {
		return nil, fmt.Errorf("parse top events: %w", err)
	}

	var topEvents []EventCount
	for _, ev := range topResp.Events {
		topEvents = append(topEvents, EventCount{Name: ev.Name, Count: ev.Count})
	}

	return &UsageSummary{
		CustomerID:  customerID,
		DAU:         engResp.Results.DAU,
		WAU:         engResp.Results.WAU,
		MAU:         engResp.Results.MAU,
		TotalEvents: engResp.Results.TotalEvents,
		TopEvents:   topEvents,
	}, nil
}

// GetFeatureAdoption returns feature adoption details for a customer.
func (m *MixpanelClient) GetFeatureAdoption(ctx context.Context, customerID string, days int) (*FeatureAdoption, error) {
	path := fmt.Sprintf("/engage/features?customer_id=%s&days=%d", customerID, days)
	data, err := m.mixpanelRequest(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get feature adoption: %w", err)
	}

	var resp mixpanelFeatureResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse feature response: %w", err)
	}

	var features []FeatureUsage
	for _, f := range resp.Features {
		lastUsed, _ := time.Parse(time.RFC3339, f.LastUsed)
		features = append(features, FeatureUsage{
			Name:        f.Name,
			EventCount:  f.EventCount,
			UniqueUsers: f.UniqueUsers,
			LastUsed:    lastUsed,
		})
	}

	return &FeatureAdoption{
		CustomerID: customerID,
		Features:   features,
	}, nil
}

// GetUsageTrend returns usage trends for a customer over the given number of months.
func (m *MixpanelClient) GetUsageTrend(ctx context.Context, customerID string, months int) (*UsageTrend, error) {
	path := fmt.Sprintf("/engage/trend?customer_id=%s&months=%d", customerID, months)
	data, err := m.mixpanelRequest(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get usage trend: %w", err)
	}

	var resp mixpanelTrendResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse trend response: %w", err)
	}

	var dataPoints []UsageDataPoint
	for _, dp := range resp.DataPoints {
		dataPoints = append(dataPoints, UsageDataPoint{
			Period:      dp.Period,
			ActiveUsers: dp.ActiveUsers,
			EventCount:  dp.EventCount,
		})
	}

	return &UsageTrend{
		CustomerID:    customerID,
		Direction:     resp.Direction,
		ChangePercent: resp.ChangePercent,
		DataPoints:    dataPoints,
	}, nil
}
