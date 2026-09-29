package views

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"webgo/internal/models"
	"webgo/internal/utils"
)

// tmpl es el set global de templates (parseado una sola vez al arrancar).
// Convención de nombres: cada archivo bajo templates/ es un template con su
// ruta relativa como nombre (ej: "layouts/admin.html", "pages/admin/pagos_list.html").
// Los layouts invocan la página activa con {{template .ActivePage .}}.
var tmpl *template.Template

// EstadoBadge devuelve un badge HTML con clases Tailwind según el estado del pago.
// Acepta any porque pgx devuelve int32 para columnas int4.
func EstadoBadge(estado any) template.HTML {
	est := utils.ToInt(estado)
	colors := map[int]string{
		51:  "bg-emerald-100 text-emerald-700",
		52:  "bg-rose-100 text-rose-700",
		53:  "bg-sky-100 text-sky-700",
		54:  "bg-amber-100 text-amber-700",
		15:  "bg-slate-100 text-slate-700",
		50:  "bg-slate-100 text-slate-700",
		0:   "bg-slate-100 text-slate-700",
		11:  "bg-slate-100 text-slate-500",
		14:  "bg-slate-100 text-slate-500",
		18:  "bg-sky-100 text-sky-700",
		55:  "bg-slate-100 text-slate-500",
		500: "bg-rose-100 text-rose-700",
	}
	color, ok := colors[est]
	if !ok {
		color = "bg-slate-100 text-slate-700"
	}
	nombre := models.EstadoNombre(est)
	return template.HTML(fmt.Sprintf(
		`<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-semibold %s">%s</span>`,
		color, template.HTMLEscapeString(nombre)))
}

// Money formatea un monto con 2 decimales.
func Money(v any) string {
	return fmt.Sprintf("%.2f", utils.ToFloat64(v))
}

// FmtFecha formatea una fecha como dd/mm/yyyy. Acepta time.Time o string.
func FmtFecha(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.Format("02/01/2006")
	case string:
		if parsed, err := time.Parse("2006-01-02T15:04:05Z", t); err == nil {
			return parsed.Format("02/01/2006")
		}
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			return parsed.Format("02/01/2006")
		}
		return t
	}
	return ""
}

// FmtFechaHora formatea fecha y hora como dd/mm/yyyy HH:MM.
func FmtFechaHora(v any) string {
	if t, ok := v.(time.Time); ok {
		return t.Format("02/01/2006 15:04")
	}
	return FmtFecha(v)
}

// FmtFechaCorta formatea como dd/mm (para gráficos diarios).
func FmtFechaCorta(v any) string {
	if t, ok := v.(time.Time); ok {
		return t.Format("02/01")
	}
	return FmtFecha(v)
}

// Init parsea todos los templates HTML bajo templates/.
func Init() error {
	funcs := template.FuncMap{
		"estadoBadge":   EstadoBadge,
		"estadoNombre":  models.EstadoNombre,
		"money":         Money,
		"fmtFecha":      FmtFecha,
		"fmtFechaHora":  FmtFechaHora,
		"fmtFechaCorta": FmtFechaCorta,
		"now":           time.Now,
		"add":           func(a, b int) int { return a + b },
		"sub":           func(a, b int) int { return a - b },
		"mul":           func(a, b int) int { return a * b },
		"div":           func(a, b int) int { return a / b },
		"seq": func(n int) []int {
			s := make([]int, 0, n)
			for i := 1; i <= n; i++ {
				s = append(s, i)
			}
			return s
		},
	}

	fsys := os.DirFS("templates")
	var files []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error recorriendo templates: %w", err)
	}

	t, err := template.New("").Funcs(funcs).ParseFS(fsys, files...)
	if err != nil {
		return fmt.Errorf("error parseando templates: %w", err)
	}
	tmpl = t
	return nil
}

// Render ejecuta el layout completo, o solo el fragmento de la página si la
// petición es HTMX (hx-target="#content").
func Render(w http.ResponseWriter, r *http.Request, layout, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("HX-Request") != "" {
		if err := tmpl.ExecuteTemplate(w, page, data); err != nil {
			http.Error(w, "Error al renderizar fragmento", http.StatusInternalServerError)
		}
		return
	}
	// Pre-renderizar la página activa e inyectarla como "PageContent" para que
	// el layout la muestre con {{.PageContent}}. "ActivePage" queda como clave
	// corta para resaltar el link del sidebar.
	if m, ok := data.(map[string]any); ok {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, page, data); err != nil {
			http.Error(w, "Error al renderizar página", http.StatusInternalServerError)
			return
		}
		m["PageContent"] = template.HTML(buf.String())
	}
	if err := tmpl.ExecuteTemplate(w, layout, data); err != nil {
		http.Error(w, "Error al renderizar página", http.StatusInternalServerError)
	}
}

// RenderFragment ejecuta solo el bloque de una página (respuesta HTMX directa).
func RenderFragment(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, page, data); err != nil {
		http.Error(w, "Error al renderizar fragmento", http.StatusInternalServerError)
	}
}

// RenderRaw ejecuta un template por nombre sin layout (ej: login standalone).
func RenderRaw(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Error al renderizar", http.StatusInternalServerError)
	}
}