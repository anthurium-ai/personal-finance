package classify

import (
	"context"
	"database/sql"
	"strings"
)

type Suggestion struct {
	Category string
	Reason   string
	Source   string // override|rule|llm
}

// SuggestCategory applies deterministic sources (override + rules).
// LLM suggestions are handled elsewhere as an optional step.
func SuggestCategory(ctx context.Context, db *sql.DB, merchantNorm string, details string) (*Suggestion, error) {
	merchantNorm = strings.TrimSpace(merchantNorm)
	details = strings.TrimSpace(details)

	// 1) explicit merchant override
	if merchantNorm != "" {
		var cat string
		err := db.QueryRowContext(ctx, `SELECT category_norm FROM merchant_category_overrides WHERE merchant_norm=?`, merchantNorm).Scan(&cat)
		if err == nil && strings.TrimSpace(cat) != "" {
			return &Suggestion{Category: cat, Reason: "merchant override", Source: "override"}, nil
		}
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}

	// 2) contains rules (simple)
	rows, err := db.QueryContext(ctx, `SELECT match_contains, category_norm FROM category_rules WHERE enabled=1 ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	text := strings.ToLower(merchantNorm + " " + details)
	for rows.Next() {
		var contains, cat string
		_ = rows.Scan(&contains, &cat)
		contains = strings.TrimSpace(contains)
		if contains == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(contains)) {
			return &Suggestion{Category: strings.TrimSpace(cat), Reason: "rule contains: " + contains, Source: "rule"}, nil
		}
	}

	return nil, nil
}
