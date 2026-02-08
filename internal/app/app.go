package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/anthurium-ai/personal-finance/internal/importer"
	"github.com/anthurium-ai/personal-finance/internal/metrics"
	"github.com/anthurium-ai/personal-finance/internal/web"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type App struct {
	DB   *sql.DB
	Tmpl *web.Templates
	Met  *metrics.Collector
}

type Config struct {
	Addr string
}

func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	// pages
	r.Get("/", a.handleUploadForm)
	r.Post("/upload", a.handleUpload)

	r.Get("/transactions", a.handleTransactions)
	r.Get("/imports", a.handleImports)

	r.Get("/tx/{id}", a.handleEditTx)
	r.Post("/tx/{id}", a.handleSaveTx)
	r.Post("/tx/{id}/suggest", a.handleSuggestTx)

	// metrics (refresh on scrape)
	r.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		_ = a.Met.Refresh(r.Context())
		promhttp.Handler().ServeHTTP(w, r)
	})
	return r
}

func (a *App) handleUploadForm(w http.ResponseWriter, r *http.Request) {
	a.Tmpl.Render(w, "upload", map[string]any{"Message": ""})
}

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	f, hdr, err := r.FormFile("file")
	if err != nil {
		a.Tmpl.Render(w, "upload", map[string]any{"Message": "missing file"})
		return
	}
	defer f.Close()

	// Limit size (25MB).
	limited := io.LimitReader(f, 25*1024*1024)
	id, total, inserted, skipped, err := importer.ImportCCCSV(a.DB, limited, hdr.Filename)
	if err != nil {
		a.Tmpl.Render(w, "upload", map[string]any{"Message": "import failed: " + err.Error()})
		return
	}
	msg := fmt.Sprintf("import #%d: rows=%d inserted=%d skipped=%d", id, total, inserted, skipped)
	a.Tmpl.Render(w, "upload", map[string]any{"Message": msg})
}

func (a *App) handleTransactions(w http.ResponseWriter, r *http.Request) {
	// KISS for now: just show latest 50.
	rows, err := a.DB.Query(`SELECT id, txn_date, amount_cents, category_norm, merchant_norm, details FROM transactions ORDER BY txn_date DESC, id DESC LIMIT 200`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	type row struct {
		ID       int64
		Date     string
		Amount   string
		Cat      string
		Merchant string
		Details  string
	}
	var out []row
	for rows.Next() {
		var id, amount int64
		var date, cat, merchant, details string
		_ = rows.Scan(&id, &date, &amount, &cat, &merchant, &details)
		out = append(out, row{ID: id, Date: date, Amount: fmtMoney(amount), Cat: cat, Merchant: merchant, Details: details})
	}

	a.Tmpl.Render(w, "transactions", map[string]any{"Rows": out})
}

func (a *App) handleImports(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT id, created_at, file_name, rows_total, rows_inserted, rows_skipped FROM imports ORDER BY id DESC LIMIT 50`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	type row struct {
		ID       int64
		Created  string
		File     string
		Total    int
		Inserted int
		Skipped  int
	}
	var out []row
	for rows.Next() {
		var id int64
		var created, file string
		var total, ins, sk int
		_ = rows.Scan(&id, &created, &file, &total, &ins, &sk)
		out = append(out, row{ID: id, Created: created, File: file, Total: total, Inserted: ins, Skipped: sk})
	}
	a.Tmpl.Render(w, "imports", map[string]any{"Rows": out})
}

func fmtMoney(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	d := cents / 100
	c := cents % 100
	return sign + "$" + strconv.FormatInt(d, 10) + "." + fmt2(c)
}

func fmt2(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}

func Run(ctx context.Context, db *sql.DB, tmpl *web.Templates, cfg Config) error {
	met := metrics.New(db)
	met.Register(prometheus.DefaultRegisterer)
	a := &App{DB: db, Tmpl: tmpl, Met: met}
	srv := &http.Server{Addr: cfg.Addr, Handler: a.Router()}

	go func() {
		<-ctx.Done()
		cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(cctx)
	}()

	return srv.ListenAndServe()
}

func DefaultDBPath() string {
	return filepath.Join("data", "finance.db")
}
