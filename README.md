# personal-finance (local)

A small, local-first personal budgeting app:
- Upload CSV exports of transactions
- Store in SQLite (deduped)
- Simple web portal for review/edit/manual adds
- Grafana for dashboards (via Prometheus metrics)

## Run

```bash
go run ./cmd/pfportal
# open http://127.0.0.1:8787
```

Database default: `data/finance.db`.

## Import

Upload a CSV from the portal home page.

Current importer supports the provided credit card CSV format with headers:
`Date,Amount,Account Number,Transaction Type,Transaction Details,Category,Merchant Name,Processed On`

## Metrics

Prometheus metrics at:
- `GET /metrics`

Grafana/Prometheus compose scaffolding TBD.

## Roadmap

- Transaction edit UI (category/merchant/notes)
- Manual transaction entry
- Classification assist (rules + learn-from-edits + optional Codex/GPT suggestion)
- Grafana dashboards + docker-compose
