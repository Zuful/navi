package chronicle

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/Zuful/navi/internal/httpclient"
)

// HubSpotClient implements CommsClient using the HubSpot API v3.
type HubSpotClient struct {
	apiKey string
	http   *httpclient.Client
}

// NewHubSpotClient creates a new HubSpot communications client.
func NewHubSpotClient(apiKey string, httpClient *httpclient.Client) *HubSpotClient {
	return &HubSpotClient{apiKey: apiKey, http: httpClient}
}

// hubspotRequest builds and executes an authenticated HubSpot API request.
func (h *HubSpotClient) hubspotRequest(ctx context.Context, path string) ([]byte, error) {
	url := "https://api.hubapi.com" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+h.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return h.http.Do(ctx, req)
}

// --- HubSpot response types ---

type hubspotSearchResults struct {
	Results []json.RawMessage `json:"results"`
	Total   int               `json:"total"`
}

type hubspotEngagement struct {
	ID         string                    `json:"id"`
	Properties hubspotEngagementProps    `json:"properties"`
	CreatedAt  string                    `json:"createdAt"`
}

type hubspotEngagementProps struct {
	Subject           string `json:"hs_email_subject"`
	Body              string `json:"hs_email_text"`
	Direction         string `json:"hs_email_direction"`
	Status            string `json:"hs_email_status"`
	Type              string `json:"hs_engagement_type"`
	Timestamp         string `json:"hs_timestamp"`
	CallTitle         string `json:"hs_call_title"`
	CallBody          string `json:"hs_call_body"`
	CallDirection     string `json:"hs_call_direction"`
	NoteBody          string `json:"hs_note_body"`
	MeetingTitle      string `json:"hs_meeting_title"`
	MeetingBody       string `json:"hs_meeting_body"`
	TaskSubject       string `json:"hs_task_subject"`
	TaskBody          string `json:"hs_task_body"`
	TaskStatus        string `json:"hs_task_status"`
}

type hubspotContact struct {
	ID         string                `json:"id"`
	Properties hubspotContactProps   `json:"properties"`
}

type hubspotContactProps struct {
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Email     string `json:"email"`
}

// GetRecentCommunications returns recent communications for a HubSpot company/contact.
func (h *HubSpotClient) GetRecentCommunications(ctx context.Context, customerID string, limit int) ([]Communication, error) {
	if limit <= 0 {
		limit = 20
	}

	var comms []Communication

	// Fetch emails.
	emails, err := h.getEngagements(ctx, customerID, "emails", limit)
	if err != nil {
		return nil, fmt.Errorf("get emails: %w", err)
	}
	for _, e := range emails {
		comms = append(comms, Communication{
			ID:        e.ID,
			Type:      "email",
			Subject:   e.Properties.Subject,
			Body:      truncateBody(e.Properties.Body, 500),
			Direction: normalizeDirection(e.Properties.Direction),
			Status:    e.Properties.Status,
			CreatedAt: parseHubSpotTime(e.CreatedAt),
		})
	}

	// Fetch calls.
	calls, err := h.getEngagements(ctx, customerID, "calls", limit)
	if err != nil {
		return nil, fmt.Errorf("get calls: %w", err)
	}
	for _, c := range calls {
		comms = append(comms, Communication{
			ID:        c.ID,
			Type:      "call",
			Subject:   c.Properties.CallTitle,
			Body:      truncateBody(c.Properties.CallBody, 500),
			Direction: normalizeDirection(c.Properties.CallDirection),
			CreatedAt: parseHubSpotTime(c.CreatedAt),
		})
	}

	// Fetch notes.
	notes, err := h.getEngagements(ctx, customerID, "notes", limit)
	if err != nil {
		return nil, fmt.Errorf("get notes: %w", err)
	}
	for _, n := range notes {
		comms = append(comms, Communication{
			ID:        n.ID,
			Type:      "note",
			Subject:   "Note",
			Body:      truncateBody(n.Properties.NoteBody, 500),
			CreatedAt: parseHubSpotTime(n.CreatedAt),
		})
	}

	// Sort by date descending.
	sort.Slice(comms, func(i, j int) bool {
		return comms[i].CreatedAt.After(comms[j].CreatedAt)
	})

	// Trim to limit.
	if len(comms) > limit {
		comms = comms[:limit]
	}

	return comms, nil
}

// GetContactTimeline returns a chronological timeline for a contact.
func (h *HubSpotClient) GetContactTimeline(ctx context.Context, contactID string, limit int) ([]TimelineEvent, error) {
	if limit <= 0 {
		limit = 30
	}

	var events []TimelineEvent

	// Get contact info.
	contactData, err := h.hubspotRequest(ctx, fmt.Sprintf("/crm/v3/objects/contacts/%s?properties=firstname,lastname,email", contactID))
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	var contact hubspotContact
	if err := json.Unmarshal(contactData, &contact); err != nil {
		return nil, fmt.Errorf("parse contact: %w", err)
	}

	// Get emails associated with contact.
	engagementTypes := []string{"emails", "calls", "notes", "meetings"}
	for _, engType := range engagementTypes {
		engs, err := h.getEngagements(ctx, contactID, engType, limit)
		if err != nil {
			continue // graceful degradation
		}
		for _, e := range engs {
			event := TimelineEvent{
				ID:        e.ID,
				Type:      engagementTypeFromEndpoint(engType),
				CreatedAt: parseHubSpotTime(e.CreatedAt),
				Metadata:  make(map[string]string),
			}

			switch engType {
			case "emails":
				event.Title = e.Properties.Subject
				event.Description = truncateBody(e.Properties.Body, 200)
				event.Metadata["direction"] = normalizeDirection(e.Properties.Direction)
			case "calls":
				event.Title = e.Properties.CallTitle
				event.Description = truncateBody(e.Properties.CallBody, 200)
				event.Metadata["direction"] = normalizeDirection(e.Properties.CallDirection)
			case "notes":
				event.Title = "Note"
				event.Description = truncateBody(e.Properties.NoteBody, 200)
			case "meetings":
				event.Title = e.Properties.MeetingTitle
				event.Description = truncateBody(e.Properties.MeetingBody, 200)
			}

			events = append(events, event)
		}
	}

	// Get associated deals.
	dealData, err := h.hubspotRequest(ctx, fmt.Sprintf("/crm/v3/objects/contacts/%s/associations/deals", contactID))
	if err == nil {
		var assocResults struct {
			Results []struct {
				ID string `json:"id"`
			} `json:"results"`
		}
		if json.Unmarshal(dealData, &assocResults) == nil {
			for _, assoc := range assocResults.Results {
				deal, err := h.hubspotRequest(ctx, fmt.Sprintf("/crm/v3/objects/deals/%s?properties=dealname,amount,dealstage,closedate,createdate", assoc.ID))
				if err != nil {
					continue
				}
				var d struct {
					ID         string `json:"id"`
					Properties struct {
						DealName  string `json:"dealname"`
						Amount    string `json:"amount"`
						DealStage string `json:"dealstage"`
						CloseDate string `json:"closedate"`
						CreateDate string `json:"createdate"`
					} `json:"properties"`
				}
				if json.Unmarshal(deal, &d) == nil {
					events = append(events, TimelineEvent{
						ID:          d.ID,
						Type:        "deal",
						Title:       d.Properties.DealName,
						Description: fmt.Sprintf("Stage: %s, Amount: %s", d.Properties.DealStage, d.Properties.Amount),
						CreatedAt:   parseHubSpotTime(d.Properties.CreateDate),
						Metadata: map[string]string{
							"stage":  d.Properties.DealStage,
							"amount": d.Properties.Amount,
						},
					})
				}
			}
		}
	}

	// Sort by date descending.
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})

	if len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

func (h *HubSpotClient) getEngagements(ctx context.Context, objectID, engType string, limit int) ([]hubspotEngagement, error) {
	path := fmt.Sprintf("/crm/v3/objects/%s?limit=%d&associations=contacts,companies", engType, limit)
	data, err := h.hubspotRequest(ctx, path)
	if err != nil {
		return nil, err
	}

	var results hubspotSearchResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse results: %w", err)
	}

	var engagements []hubspotEngagement
	for _, raw := range results.Results {
		var eng hubspotEngagement
		if json.Unmarshal(raw, &eng) == nil {
			engagements = append(engagements, eng)
		}
	}

	return engagements, nil
}

func parseHubSpotTime(s string) time.Time {
	// HubSpot uses ISO 8601 format.
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try millisecond timestamp.
		t, err = time.Parse("2006-01-02T15:04:05.000Z", s)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

func normalizeDirection(d string) string {
	switch d {
	case "INCOMING_EMAIL", "INBOUND", "inbound":
		return "inbound"
	case "FORWARDED_EMAIL", "OUTGOING_EMAIL", "OUTBOUND", "outbound":
		return "outbound"
	default:
		return d
	}
}

func engagementTypeFromEndpoint(endpoint string) string {
	switch endpoint {
	case "emails":
		return "email"
	case "calls":
		return "call"
	case "notes":
		return "note"
	case "meetings":
		return "meeting"
	default:
		return endpoint
	}
}

func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
