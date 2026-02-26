# Navi

Customer Success MCP server that aggregates customer signals from external SaaS tools (Stripe, HubSpot, Zendesk) so AI agents can answer CS questions like "which accounts are at risk?" or "show me this customer's billing history."

Named after the fairy companion from Zelda.

## Tools

### Vault (Billing — Stripe)
- `get_subscription_status` — subscription plan, status, MRR, trial info
- `get_revenue_metrics` — MRR history, payment success rate, outstanding invoices
- `get_renewal_calendar` — upcoming renewals/expirations within N days

### Chronicle (Communications — HubSpot)
- `get_recent_communications` — recent emails, notes, calls for a customer
- `get_contact_timeline` — chronological activity timeline for a contact

### Beacon (Support Tickets — Zendesk)
- `get_open_tickets` — open/pending support tickets for a customer
- `get_ticket_history` — full ticket history across all statuses
- `get_satisfaction_scores` — CSAT scores and resolution metrics

### Pulse (Health Scoring)
- `get_customer_health` — composite health score from billing + communications signals
- `list_at_risk_accounts` — accounts with warning signals across providers

### Scout (Churn Prediction)
- `get_churn_risk` — churn risk assessment with contributing factors

## Setup

```bash
# Build
go build ./cmd/navi

# Configure
cp navi.yaml.example navi.yaml
export NAVI_VAULT_API_KEY="sk_..."        # Stripe secret key
export NAVI_CHRONICLE_API_KEY="pat-..."   # HubSpot private app token
export NAVI_BEACON_API_KEY="..."         # Zendesk API token
export NAVI_BEACON_SUBDOMAIN="mycompany" # Zendesk subdomain

# Run
./navi
```

## Configuration

Navi uses YAML configuration with environment variable overrides:

| Env Var | Description |
|---------|-------------|
| `NAVI_CONFIG` | Path to YAML config file (default: `navi.yaml`) |
| `NAVI_LOG_LEVEL` | Log level: debug, info, warn, error |
| `NAVI_VAULT_API_KEY` | Stripe API secret key |
| `NAVI_VAULT_BACKEND` | Billing backend (default: `stripe`) |
| `NAVI_CHRONICLE_API_KEY` | HubSpot private app token |
| `NAVI_CHRONICLE_BACKEND` | Communications backend (default: `hubspot`) |
| `NAVI_BEACON_API_KEY` | Zendesk API token |
| `NAVI_BEACON_BACKEND` | Support ticket backend (default: `zendesk`) |
| `NAVI_BEACON_SUBDOMAIN` | Zendesk subdomain (e.g. `mycompany`) |

Providers are optional — omit the config section or API key to skip a provider. Aggregators (Pulse, Scout) always register and work with whatever providers are available.

## Docker

```bash
docker build -t navi .
docker run -e NAVI_VAULT_API_KEY=sk_... -e NAVI_CHRONICLE_API_KEY=pat-... -e NAVI_BEACON_API_KEY=... -e NAVI_BEACON_SUBDOMAIN=mycompany navi
```

## MCP Client Configuration

```json
{
  "mcpServers": {
    "navi": {
      "command": "./navi",
      "env": {
        "NAVI_VAULT_API_KEY": "sk_...",
        "NAVI_CHRONICLE_API_KEY": "pat-...",
        "NAVI_BEACON_API_KEY": "...",
        "NAVI_BEACON_SUBDOMAIN": "mycompany"
      }
    }
  }
}
```
