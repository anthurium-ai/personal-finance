package app

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/anthurium-ai/personal-finance/internal/classify"
	"github.com/go-chi/chi/v5"
)

type txView struct {
	ID          int64
	Date        string
	Amount      string
	Category    string
	Merchant    string
	MerchantRaw string
	Details     string
	Notes       string
}

type suggestionView struct {
	Category string
	Reason   string
	Source   string
}

func (a *App) handleEditTx(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var t txView
	var amountCents int64
	row := a.DB.QueryRow(`
		SELECT id, txn_date, amount_cents,
		       COALESCE(NULLIF(category_norm,''), category_raw),
		       COALESCE(NULLIF(merchant_norm,''), merchant_raw),
		       merchant_raw, details, COALESCE(notes,'')
		FROM transactions WHERE id=?`, id)
	if err := row.Scan(&t.ID, &t.Date, &amountCents, &t.Category, &t.Merchant, &t.MerchantRaw, &t.Details, &t.Notes); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	t.Amount = fmtMoney(amountCents)

	// deterministic suggestion
	sug, _ := classify.SuggestCategory(r.Context(), a.DB, t.Merchant, t.Details)
	var sv *suggestionView
	if sug != nil {
		sv = &suggestionView{Category: sug.Category, Reason: sug.Reason, Source: sug.Source}
	}
	qs := r.URL.Query()
	if qs.Get("suggest") != "" {
		sv = &suggestionView{Category: qs.Get("suggest"), Reason: qs.Get("reason"), Source: qs.Get("source")}
	}

	a.Tmpl.Render(w, "edit_tx", map[string]any{"Tx": t, "Suggestion": sv})
}

func (a *App) handleSaveTx(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	_ = r.ParseForm()
	cat := strings.TrimSpace(r.FormValue("category_norm"))
	mer := strings.TrimSpace(r.FormValue("merchant_norm"))	
	notes := strings.TrimSpace(r.FormValue("notes"))

	_, err := a.DB.Exec(`UPDATE transactions SET category_norm=?, merchant_norm=?, notes=? WHERE id=?`, cat, mer, notes, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// learn override: merchant -> category
	if mer != "" && cat != "" {
		_, _ = a.DB.Exec(`INSERT INTO merchant_category_overrides (merchant_norm, category_norm) VALUES (?,?) ON CONFLICT(merchant_norm) DO UPDATE SET category_norm=excluded.category_norm`, mer, cat)
	}

	http.Redirect(w, r, "/tx/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) handleSuggestTx(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var merchantRaw, details string
	var amountCents int64
	row := a.DB.QueryRow(`SELECT merchant_raw, details, amount_cents FROM transactions WHERE id=?`, id)
	if err := row.Scan(&merchantRaw, &details, &amountCents); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	// gather some known categories for the prompt
	catRows, _ := a.DB.Query(`SELECT DISTINCT COALESCE(NULLIF(category_norm,''), category_raw) FROM transactions WHERE COALESCE(NULLIF(category_norm,''), category_raw) != '' LIMIT 50`)
	var cats []string
	for catRows != nil && catRows.Next() {
		var c string
		_ = catRows.Scan(&c)
		cats = append(cats, c)
	}
	if catRows != nil {
		catRows.Close()
	}

	sug, err := classify.SuggestCategoryLLM(r.Context(), merchantRaw, details, amountCents, cats)
	if err != nil {
		http.Redirect(w, r, "/tx/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}

	// show suggestion by re-rendering edit page with suggestion
	http.Redirect(w, r, "/tx/"+strconv.FormatInt(id, 10)+"?suggest="+urlQueryEscape(sug.Category)+"&reason="+urlQueryEscape(sug.Reason)+"&source=llm", http.StatusSeeOther)
}

func urlQueryEscape(s string) string {
	r := strings.NewReplacer("%", "%25", " ", "%20", "\n", "%0A", "\r", "")
	return r.Replace(s)
}
