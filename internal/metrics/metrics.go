package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	db *sql.DB

	spendByCategory *prometheus.GaugeVec
	incomeTotal     prometheus.Gauge
	expenseTotal    prometheus.Gauge
}

func New(db *sql.DB) *Collector {
	c := &Collector{db: db}
	c.spendByCategory = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "spend_by_category_mtd_cents",
		Help:      "Month-to-date spend by category in cents (negative amounts are spend)",
	}, []string{"category"})

	c.incomeTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "income_mtd_cents",
		Help:      "Month-to-date income in cents",
	})
	c.expenseTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "expense_mtd_cents",
		Help:      "Month-to-date expenses in cents",
	})
	return c
}

func (c *Collector) Register(reg prometheus.Registerer) {
	reg.MustRegister(c.spendByCategory, c.incomeTotal, c.expenseTotal)
}

// Refresh recomputes gauges (call on each scrape or periodically).
func (c *Collector) Refresh(ctx context.Context) error {
	start := monthStart(time.Now())

	// reset by category
	c.spendByCategory.Reset()

	rows, err := c.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(category_norm,''), COALESCE(NULLIF(category_raw,''),'Uncategorised')) as cat,
		       SUM(CASE WHEN amount_cents < 0 THEN -amount_cents ELSE 0 END) as spend
		FROM transactions
		WHERE txn_date >= ?
		GROUP BY 1
		ORDER BY spend DESC
	`, start.Format("2006-01-02"))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cat string
		var spend int64
		_ = rows.Scan(&cat, &spend)
		c.spendByCategory.WithLabelValues(cat).Set(float64(spend))
	}

	var income, expense int64
	err = c.db.QueryRowContext(ctx, `
		SELECT
		  SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END) as income,
		  SUM(CASE WHEN amount_cents < 0 THEN -amount_cents ELSE 0 END) as expense
		FROM transactions
		WHERE txn_date >= ?
	`, start.Format("2006-01-02")).Scan(&income, &expense)
	if err != nil {
		return err
	}
	c.incomeTotal.Set(float64(income))
	c.expenseTotal.Set(float64(expense))
	return nil
}

func monthStart(t time.Time) time.Time {
	loc := t.Location()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
}

func HumanMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}
