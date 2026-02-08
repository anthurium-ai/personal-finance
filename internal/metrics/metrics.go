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

	spendByCategoryMTD *prometheus.GaugeVec
	incomeMTD          prometheus.Gauge
	expenseMTD         prometheus.Gauge

	// Multi-month series (last N months, inclusive of current)
	spendByCategoryByMonth *prometheus.GaugeVec
	expenseByMonth         *prometheus.GaugeVec
	incomeByMonth          *prometheus.GaugeVec

	// Top merchants (MTD)
	spendByMerchantMTD *prometheus.GaugeVec
}

func New(db *sql.DB) *Collector {
	c := &Collector{db: db}

	c.spendByCategoryMTD = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "spend_by_category_mtd_cents",
		Help:      "Month-to-date spend by category in cents",
	}, []string{"category"})

	c.incomeMTD = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "income_mtd_cents",
		Help:      "Month-to-date income in cents",
	})
	c.expenseMTD = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "expense_mtd_cents",
		Help:      "Month-to-date expenses in cents",
	})

	c.spendByCategoryByMonth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "spend_by_category_month_cents",
		Help:      "Spend by category per month in cents (month label is YYYY-MM)",
	}, []string{"month", "category"})

	c.expenseByMonth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "expense_month_cents",
		Help:      "Expenses per month in cents (month label is YYYY-MM)",
	}, []string{"month"})

	c.incomeByMonth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "income_month_cents",
		Help:      "Income per month in cents (month label is YYYY-MM)",
	}, []string{"month"})

	c.spendByMerchantMTD = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pf",
		Name:      "spend_by_merchant_mtd_cents",
		Help:      "Month-to-date spend by merchant in cents (top N only)",
	}, []string{"merchant"})

	return c
}

func (c *Collector) Register(reg prometheus.Registerer) {
	reg.MustRegister(
		c.spendByCategoryMTD,
		c.incomeMTD,
		c.expenseMTD,
		c.spendByCategoryByMonth,
		c.expenseByMonth,
		c.incomeByMonth,
		c.spendByMerchantMTD,
	)
}

// Refresh recomputes gauges (call on each scrape or periodically).
func (c *Collector) Refresh(ctx context.Context) error {
	start := monthStart(time.Now())

	// --- MTD ---
	c.spendByCategoryMTD.Reset()
	c.spendByMerchantMTD.Reset()

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
	for rows.Next() {
		var cat string
		var spend int64
		_ = rows.Scan(&cat, &spend)
		c.spendByCategoryMTD.WithLabelValues(cat).Set(float64(spend))
	}
	rows.Close()

	mrows, err := c.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(merchant_norm,''), COALESCE(NULLIF(merchant_raw,''),'Unknown')) as mer,
		       SUM(CASE WHEN amount_cents < 0 THEN -amount_cents ELSE 0 END) as spend
		FROM transactions
		WHERE txn_date >= ?
		GROUP BY 1
		ORDER BY spend DESC
		LIMIT 15
	`, start.Format("2006-01-02"))
	if err != nil {
		return err
	}
	for mrows.Next() {
		var mer string
		var spend int64
		_ = mrows.Scan(&mer, &spend)
		c.spendByMerchantMTD.WithLabelValues(mer).Set(float64(spend))
	}
	mrows.Close()

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
	c.incomeMTD.Set(float64(income))
	c.expenseMTD.Set(float64(expense))

	// --- Last N months ---
	c.spendByCategoryByMonth.Reset()
	c.incomeByMonth.Reset()
	c.expenseByMonth.Reset()

	months := lastNMonths(time.Now(), 6)
	for _, m := range months {
		from := time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, m.Location())
		to := from.AddDate(0, 1, 0)
		label := from.Format("2006-01")

		var inc, exp int64
		err = c.db.QueryRowContext(ctx, `
			SELECT
			  SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END) as income,
			  SUM(CASE WHEN amount_cents < 0 THEN -amount_cents ELSE 0 END) as expense
			FROM transactions
			WHERE txn_date >= ? AND txn_date < ?
		`, from.Format("2006-01-02"), to.Format("2006-01-02")).Scan(&inc, &exp)
		if err != nil {
			return err
		}
		c.incomeByMonth.WithLabelValues(label).Set(float64(inc))
		c.expenseByMonth.WithLabelValues(label).Set(float64(exp))

		crows, err := c.db.QueryContext(ctx, `
			SELECT COALESCE(NULLIF(category_norm,''), COALESCE(NULLIF(category_raw,''),'Uncategorised')) as cat,
			       SUM(CASE WHEN amount_cents < 0 THEN -amount_cents ELSE 0 END) as spend
			FROM transactions
			WHERE txn_date >= ? AND txn_date < ?
			GROUP BY 1
			ORDER BY spend DESC
		`, from.Format("2006-01-02"), to.Format("2006-01-02"))
		if err != nil {
			return err
		}
		for crows.Next() {
			var cat string
			var spend int64
			_ = crows.Scan(&cat, &spend)
			c.spendByCategoryByMonth.WithLabelValues(label, cat).Set(float64(spend))
		}
		crows.Close()
	}

	return nil
}

func monthStart(t time.Time) time.Time {
	loc := t.Location()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
}

func lastNMonths(t time.Time, n int) []time.Time {
	if n <= 0 {
		return nil
	}
	out := make([]time.Time, 0, n)
	// start from current month, go backwards
	cur := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	for i := 0; i < n; i++ {
		out = append(out, cur.AddDate(0, -i, 0))
	}
	// reverse so charts read oldest->newest
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func HumanMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}
