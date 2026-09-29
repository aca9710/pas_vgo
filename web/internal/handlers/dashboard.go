package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"webgo/internal/db"
	"webgo/internal/middleware"
	"webgo/internal/models"
	"webgo/internal/utils"
	"webgo/internal/views"
)

// Dashboard muestra el dashboard principal del admin.
// Equivalente a dashboard() de pagos_admin.py.
func Dashboard(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Stats de los últimos 30 días (globales para admin, filtradas por tienda si no)
	stats := map[string]any{
		"total_pagos":    int64(0),
		"pagos_exitosos": int64(0),
		"monto_total":    float64(0),
		"pagos_proceso":  int64(0),
	}

	statsQuery := `
		SELECT
			COUNT(*) as total_pagos,
			COUNT(*) FILTER (WHERE estado IN (51, 53)) as pagos_exitosos,
			COALESCE(SUM(importe) FILTER (WHERE estado IN (51, 53)), 0) as monto_total,
			COUNT(*) FILTER (WHERE estado = 15) as pagos_proceso
		FROM pagos
		WHERE fecha >= CURRENT_DATE - INTERVAL '30 days'`

	var args []any
	consultarStats := true

	if admin.Rol != "admin" {
		if len(admin.Tiendas) > 0 {
			// Filtro por tiendas asignadas con placeholders dinámicos ($1, $2, ...)
			placeholders := make([]string, len(admin.Tiendas))
			args = make([]any, len(admin.Tiendas))
			for i, tienda := range admin.Tiendas {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
				args[i] = tienda
			}
			statsQuery += fmt.Sprintf(" AND uid IN (%s)", strings.Join(placeholders, ","))
		} else {
			// Sin tiendas asignadas: no hay pagos visibles, stats en cero
			consultarStats = false
		}
	}

	if consultarStats {
		if row, err := db.Instance.FetchOne(r.Context(), statsQuery, args...); err != nil {
			utils.LogError("dashboard_stats")
		} else if row != nil {
			stats = row
		}
	}

	// Últimos 20 pagos con el texto de la URL asociada
	ultimosPagos := []map[string]any{}
	rows, err := db.Instance.Fetch(r.Context(), `
		SELECT p.*, u.url as url_texto
		FROM pagos p
		LEFT JOIN url u ON p.url = u.id
		ORDER BY p.fecha DESC
		LIMIT 20
	`)
	if err != nil {
		utils.LogError("dashboard_ultimos_pagos")
	} else {
		ultimosPagos = rows
	}

	// Distribución de estados (top 5) para el gráfico
	estadosLabels := []string{}
	estadosData := []int64{}
	estadosDist, err := db.Instance.Fetch(r.Context(), `
		SELECT estado, COUNT(*) as total
		FROM pagos
		GROUP BY estado
		ORDER BY total DESC
		LIMIT 5
	`)
	if err != nil {
		utils.LogError("dashboard_estados_dist")
	} else {
		for _, row := range estadosDist {
			estado := utils.ToInt(row["estado"])
			estadosLabels = append(estadosLabels, models.EstadoNombre(estado))
			estadosData = append(estadosData, utils.ToInt64(row["total"]))
		}
	}

	views.Render(w, r, "layouts/admin.html", "pages/admin/dashboard.html", map[string]any{
		"Title":         "Dashboard",
		"ActivePage":    "dashboard",
		"Admin":         admin,
		"Stats":         stats,
		"UltimosPagos":  ultimosPagos,
		"EstadosLabels": estadosLabels,
		"EstadosData":   estadosData,
		"Now":           time.Now(),
	})
}