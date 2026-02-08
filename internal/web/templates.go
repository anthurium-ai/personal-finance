package web

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Templates struct {
	base *template.Template
}

func LoadTemplates() (*Templates, error) {
	t, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Templates{base: t}, nil
}

func (t *Templates) Render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.base.ExecuteTemplate(w, name, data)
}
