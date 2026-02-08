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

Included metrics:
- `pf_spend_by_category_mtd_cents{category}`
- `pf_spend_by_merchant_mtd_cents{merchant}` (top 15)
- `pf_income_mtd_cents`
- `pf_expense_mtd_cents`
- `pf_income_month_cents{month}` (last 6 months)
- `pf_expense_month_cents{month}` (last 6 months)
- `pf_spend_by_category_month_cents{month,category}` (last 6 months)

## Grafana dashboards

Bring up Grafana + Prometheus:

```bash
docker compose up
```

- Grafana: http://127.0.0.1:3000 (admin/admin)
- Prometheus: http://127.0.0.1:9090

Dashboards are provisioned from `ops/grafana/dashboards/`.

Note: Prometheus scrapes `host.docker.internal:8787` (works on Mac/Windows; Linux may need an adjustment).

## Roadmap

- Transaction edit UI (category/merchant/notes)
- Manual transaction entry
- Classification assist (rules + learn-from-edits + optional Codex/GPT suggestion)
- Grafana dashboards + docker-compose
