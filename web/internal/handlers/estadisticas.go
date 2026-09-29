package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"

	"webgo/internal/db"
	"webgo/internal/middleware"
	"webgo/internal/models"
	"webgo/internal/utils"
	"webgo/internal/views"
)

// calcularTotales suma las filas de datos y retorna un mapa con los totales.
func calcularTotales(data []map[string]any) map[string]any {
	var pagosCount int64
	var pagosMonto, devMonto float64
	var devCount int64

	for _, d := range data {
		pagosCount += utils.ToInt64(d["pagos_count"])
		pagosMonto += utils.ToFloat64(d["pagos_monto"])
		devCount += utils.ToInt64(d["dev_count"])
		devMonto += utils.ToFloat64(d["dev_monto"])
	}

	return map[string]any{
		"pagos_count": pagosCount,
		"pagos_monto": pagosMonto,
		"dev_count":   devCount,
		"dev_monto":   devMonto,
		"neto_monto":  pagosMonto - devMonto,
	}
}

// getResumen calcula el resumen general: pagos, devoluciones y neto.
func getResumen(dataPagos, dataDev []map[string]any) map[string]any {
	var pagosCount int64
	var pagosMonto float64
	for _, d := range dataPagos {
		pagosCount += utils.ToInt64(d["pagos_count"])
		pagosMonto += utils.ToFloat64(d["pagos_monto"])
	}

	var devCount int64
	var devMonto float64
	for _, d := range dataDev {
		devCount += utils.ToInt64(d["dev_count"])
		devMonto += utils.ToFloat64(d["dev_monto"])
	}

	return map[string]any{
		"pagos": map[string]any{
			"cantidad": pagosCount,
			"monto":    pagosMonto,
		},
		"devoluciones": map[string]any{
			"cantidad": devCount,
			"monto":    devMonto,
		},
		"neto": map[string]any{
			"cantidad": pagosCount - devCount,
			"monto":    pagosMonto - devMonto,
		},
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Consultas por tipo
// ────────────────────────────────────────────────────────────────────────────

// estadisticasMeses consulta pagos y devoluciones agrupados por mes del año.
func estadisticasMeses(ctx context.Context, anio int) ([]map[string]any, []map[string]any, []map[string]any) {
	// Pagos por mes
	pagosRows, err := db.Instance.Fetch(ctx,
		`SELECT EXTRACT(MONTH FROM fecha)::int as mes_num,
		        COUNT(*) as pagos_count,
		        COALESCE(SUM(importe),0) as pagos_monto
		 FROM pagos
		 WHERE EXTRACT(YEAR FROM fecha) = $1
		 GROUP BY EXTRACT(MONTH FROM fecha)
		 ORDER BY mes_num`, anio)
	if err != nil {
		utils.LogError("estadisticas_pagos_meses")
		pagosRows = nil
	}

	// Devoluciones por mes
	devRows, err := db.Instance.Fetch(ctx,
		`SELECT EXTRACT(MONTH FROM fecha)::int as mes_num,
		        COUNT(*) as dev_count,
		        COALESCE(SUM(importe),0) as dev_monto
		 FROM devolucion
		 WHERE EXTRACT(YEAR FROM fecha) = $1
		 GROUP BY EXTRACT(MONTH FROM fecha)
		 ORDER BY mes_num`, anio)
	if err != nil {
		utils.LogError("estadisticas_dev_meses")
		devRows = nil
	}

	// Indexar por número de mes
	pagosMap := make(map[int]map[string]any)
	for _, row := range pagosRows {
		pagosMap[utils.ToInt(row["mes_num"])] = row
	}
	devMap := make(map[int]map[string]any)
	for _, row := range devRows {
		devMap[utils.ToInt(row["mes_num"])] = row
	}

	// Combinar en 12 filas (mes 1 a 12)
	datos := make([]map[string]any, 0, 12)
	for mes := 1; mes <= 12; mes++ {
		pagos := pagosMap[mes] // nil si no hay datos → utils.ToFloat64(nil) = 0
		dev := devMap[mes]
		pagosMonto := utils.ToFloat64(pagos["pagos_monto"])
		devMonto := utils.ToFloat64(dev["dev_monto"])
		datos = append(datos, map[string]any{
			"etiqueta":    models.MesesES[mes],
			"pagos_count": utils.ToInt64(pagos["pagos_count"]),
			"pagos_monto": pagosMonto,
			"dev_count":   utils.ToInt64(dev["dev_count"]),
			"dev_monto":   devMonto,
			"neto_monto":  pagosMonto - devMonto,
		})
	}

	return datos, pagosRows, devRows
}

// estadisticasDias consulta pagos y devoluciones agrupados por día.
func estadisticasDias(ctx context.Context, fechaDesde, fechaHasta string) ([]map[string]any, []map[string]any, []map[string]any) {
	// Pagos por día
	pagosRows, err := db.Instance.Fetch(ctx,
		`SELECT fecha::date as dia,
		        COUNT(*) as pagos_count,
		        COALESCE(SUM(importe),0) as pagos_monto
		 FROM pagos
		 WHERE fecha::date BETWEEN $1 AND $2
		 GROUP BY fecha::date
		 ORDER BY dia`, fechaDesde, fechaHasta)
	if err != nil {
		utils.LogError("estadisticas_pagos_dias")
		pagosRows = nil
	}

	// Devoluciones por día
	devRows, err := db.Instance.Fetch(ctx,
		`SELECT fecha::date as dia,
		        COUNT(*) as dev_count,
		        COALESCE(SUM(importe),0) as dev_monto
		 FROM devolucion
		 WHERE fecha::date BETWEEN $1 AND $2
		 GROUP BY fecha::date
		 ORDER BY dia`, fechaDesde, fechaHasta)
	if err != nil {
		utils.LogError("estadisticas_dev_dias")
		devRows = nil
	}

	// Función auxiliar para extraer la clave de fecha desde el mapa
	fechaKey := func(row map[string]any) string {
		if t, ok := row["dia"].(time.Time); ok {
			return t.Format("2006-01-02")
		}
		return fmt.Sprintf("%v", row["dia"])
	}

	// Indexar por fecha
	pagosMap := make(map[string]map[string]any)
	for _, row := range pagosRows {
		pagosMap[fechaKey(row)] = row
	}
	devMap := make(map[string]map[string]any)
	for _, row := range devRows {
		devMap[fechaKey(row)] = row
	}

	// Generar filas para cada día del rango (incluye días sin datos)
	fInicio, _ := time.Parse("2006-01-02", fechaDesde)
	fFin, _ := time.Parse("2006-01-02", fechaHasta)

	datos := make([]map[string]any, 0)
	for d := fInicio; !d.After(fFin); d = d.AddDate(0, 0, 1) {
		fs := d.Format("2006-01-02")
		pagos := pagosMap[fs]
		dev := devMap[fs]
		pagosMonto := utils.ToFloat64(pagos["pagos_monto"])
		devMonto := utils.ToFloat64(dev["dev_monto"])
		datos = append(datos, map[string]any{
			"etiqueta":    d.Format("02/01/2006"),
			"pagos_count": utils.ToInt64(pagos["pagos_count"]),
			"pagos_monto": pagosMonto,
			"dev_count":   utils.ToInt64(dev["dev_count"]),
			"dev_monto":   devMonto,
			"neto_monto":  pagosMonto - devMonto,
		})
	}

	return datos, pagosRows, devRows
}

// estadisticasTiendas consulta pagos y devoluciones agrupados por tienda.
func estadisticasTiendas(ctx context.Context) ([]map[string]any, []map[string]any, []map[string]any) {
	// Pagos por tienda (JOIN con tiendas para obtener nombre)
	pagosRows, err := db.Instance.Fetch(ctx,
		`SELECT t.uid, t.nombre,
		        COUNT(p.id) as pagos_count,
		        COALESCE(SUM(p.importe),0) as pagos_monto
		 FROM tiendas t
		 LEFT JOIN pagos p ON t.uid = p.uid
		 GROUP BY t.uid, t.nombre
		 ORDER BY t.nombre`)
	if err != nil {
		utils.LogError("estadisticas_pagos_tiendas")
		pagosRows = nil
	}

	// Devoluciones por tienda
	devRows, err := db.Instance.Fetch(ctx,
		`SELECT t.uid, t.nombre,
		        COUNT(d.id) as dev_count,
		        COALESCE(SUM(d.importe),0) as dev_monto
		 FROM tiendas t
		 LEFT JOIN devolucion d ON t.uid = d.uid
		 GROUP BY t.uid, t.nombre
		 ORDER BY t.nombre`)
	if err != nil {
		utils.LogError("estadisticas_dev_tiendas")
		devRows = nil
	}

	// Combinar datos pagos + devoluciones por uid
	type tiendaCombo struct {
		UID    string
		Nombre string
		PagosC int64
		PagosM float64
		DevC   int64
		DevM   float64
	}
	combos := make(map[string]*tiendaCombo)

	for _, row := range pagosRows {
		uid := utils.ToString(row["uid"])
		combos[uid] = &tiendaCombo{
			UID:    uid,
			Nombre: utils.ToString(row["nombre"]),
			PagosC: utils.ToInt64(row["pagos_count"]),
			PagosM: utils.ToFloat64(row["pagos_monto"]),
		}
	}
	for _, row := range devRows {
		uid := utils.ToString(row["uid"])
		c, ok := combos[uid]
		if !ok {
			c = &tiendaCombo{
				UID:    uid,
				Nombre: utils.ToString(row["nombre"]),
			}
			combos[uid] = c
		}
		c.DevC = utils.ToInt64(row["dev_count"])
		c.DevM = utils.ToFloat64(row["dev_monto"])
	}

	// Ordenar por nombre
	uids := make([]string, 0, len(combos))
	for uid := range combos {
		uids = append(uids, uid)
	}
	sort.Slice(uids, func(i, j int) bool {
		return combos[uids[i]].Nombre < combos[uids[j]].Nombre
	})

	datos := make([]map[string]any, 0, len(uids))
	for _, uid := range uids {
		c := combos[uid]
		datos = append(datos, map[string]any{
			"etiqueta":    fmt.Sprintf("%s — %s", c.UID, c.Nombre),
			"pagos_count": c.PagosC,
			"pagos_monto": c.PagosM,
			"dev_count":   c.DevC,
			"dev_monto":   c.DevM,
			"neto_monto":  c.PagosM - c.DevM,
		})
	}

	return datos, pagosRows, devRows
}

// ────────────────────────────────────────────────────────────────────────────
// Handler principal: Dashboard de estadísticas
// ────────────────────────────────────────────────────────────────────────────

// Estadisticas muestra el dashboard de estadísticas del admin.
// GET /admin/estadisticas
func Estadisticas(w http.ResponseWriter, r *http.Request) {
	admin := middleware.AdminFromContext(r)
	if admin == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Parámetros de query
	tipo := r.URL.Query().Get("tipo")
	if tipo == "" {
		tipo = "meses"
	}

	anio := time.Now().Year()
	if s := r.URL.Query().Get("anio"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			anio = v
		}
	}

	fechaDesde := r.URL.Query().Get("fecha_desde")
	fechaHasta := r.URL.Query().Get("fecha_hasta")

	ctx := r.Context()
	var datos []map[string]any
	var dataPagos, dataDev []map[string]any

	switch tipo {
	case "dias":
		// Default: desde primer día del mes hasta hoy
		now := time.Now()
		if fechaDesde == "" {
			fechaDesde = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format("2006-01-02")
		}
		if fechaHasta == "" {
			fechaHasta = now.Format("2006-01-02")
		}
		datos, dataPagos, dataDev = estadisticasDias(ctx, fechaDesde, fechaHasta)

	case "tiendas":
		datos, dataPagos, dataDev = estadisticasTiendas(ctx)

	default: // "meses" u otro valor
		datos, dataPagos, dataDev = estadisticasMeses(ctx, anio)
		tipo = "meses"
	}

	totales := calcularTotales(datos)
	resumen := getResumen(dataPagos, dataDev)

	views.Render(w, r, "layouts/admin.html", "pages/admin/estadisticas.html", map[string]any{
		"Title":      "Estadísticas",
		"ActivePage": "estadisticas",
		"Admin":      admin,
		"Tipo":       tipo,
		"Anio":       anio,
		"FechaDesde": fechaDesde,
		"FechaHasta": fechaHasta,
		"Datos":      datos,
		"Totales":    totales,
		"Resumen":    resumen,
		"Now":        time.Now(),
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Handler: Reporte PDF
// ────────────────────────────────────────────────────────────────────────────

// ReportePDF genera un reporte en PDF de transacciones.
// GET /admin/estadisticas/reporte_pdf
func ReportePDF(w http.ResponseWriter, r *http.Request) {
	tipo := r.URL.Query().Get("tipo")
	if tipo == "" {
		http.Error(w, "Parámetro 'tipo' requerido (meses|dias|tiendas)", http.StatusBadRequest)
		return
	}

	anio := time.Now().Year()
	if s := r.URL.Query().Get("anio"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			anio = v
		}
	}

	fechaDesde := r.URL.Query().Get("fecha_desde")
	fechaHasta := r.URL.Query().Get("fecha_hasta")

	ctx := r.Context()
	var datos []map[string]any

	switch tipo {
	case "dias":
		now := time.Now()
		if fechaDesde == "" {
			fechaDesde = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format("2006-01-02")
		}
		if fechaHasta == "" {
			fechaHasta = now.Format("2006-01-02")
		}
		datos, _, _ = estadisticasDias(ctx, fechaDesde, fechaHasta)

	case "tiendas":
		datos, _, _ = estadisticasTiendas(ctx)

	default: // "meses"
		datos, _, _ = estadisticasMeses(ctx, anio)
		tipo = "meses"
	}

	// Generar PDF con gofpdf
	titulos := map[string]string{
		"meses":   "Reporte por Meses",
		"dias":    "Reporte por Día",
		"tiendas": "Reporte por Tienda",
	}
	tituloPDF := titulos[tipo]
	if tituloPDF == "" {
		tituloPDF = "Reporte de Transacciones"
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Encabezado: título
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, tituloPDF)
	pdf.Ln(8)

	// Subtítulo: fecha de generación
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Generado: %s", time.Now().Format("02/01/2006 15:04:05")))
	pdf.Ln(6)

	// Subtítulo: parámetros del reporte
	if tipo == "meses" {
		pdf.Cell(0, 6, fmt.Sprintf("Año: %d", anio))
	} else {
		pdf.Cell(0, 6, fmt.Sprintf("Desde: %s  Hasta: %s", fechaDesde, fechaHasta))
	}
	pdf.Ln(12)

	// Definir columnas según tipo
	type colDef struct {
		Titulo string
		Ancho  float64
		Clave  string // clave del mapa de datos
	}

	var cols []colDef
	anchoEtiqueta := 50.0
	anchoNum := 28.0
	anchoMonto := 30.0

	switch tipo {
	case "meses":
		cols = []colDef{
			{Titulo: "Mes", Ancho: anchoEtiqueta, Clave: "etiqueta"},
			{Titulo: "Pagos Cant.", Ancho: anchoNum, Clave: "pagos_count"},
			{Titulo: "Pagos Monto", Ancho: anchoMonto, Clave: "pagos_monto"},
			{Titulo: "Dev. Cant.", Ancho: anchoNum, Clave: "dev_count"},
			{Titulo: "Dev. Monto", Ancho: anchoMonto, Clave: "dev_monto"},
			{Titulo: "Neto", Ancho: anchoMonto, Clave: "neto_monto"},
		}
	case "dias":
		cols = []colDef{
			{Titulo: "Fecha", Ancho: anchoEtiqueta, Clave: "etiqueta"},
			{Titulo: "Pagos Cant.", Ancho: anchoNum, Clave: "pagos_count"},
			{Titulo: "Pagos Monto", Ancho: anchoMonto, Clave: "pagos_monto"},
			{Titulo: "Dev. Cant.", Ancho: anchoNum, Clave: "dev_count"},
			{Titulo: "Dev. Monto", Ancho: anchoMonto, Clave: "dev_monto"},
			{Titulo: "Neto", Ancho: anchoMonto, Clave: "neto_monto"},
		}
	case "tiendas":
		cols = []colDef{
			{Titulo: "Tienda", Ancho: anchoEtiqueta, Clave: "etiqueta"},
			{Titulo: "Pagos Cant.", Ancho: anchoNum, Clave: "pagos_count"},
			{Titulo: "Pagos Monto", Ancho: anchoMonto, Clave: "pagos_monto"},
			{Titulo: "Dev. Cant.", Ancho: anchoNum, Clave: "dev_count"},
			{Titulo: "Dev. Monto", Ancho: anchoMonto, Clave: "dev_monto"},
			{Titulo: "Neto", Ancho: anchoMonto, Clave: "neto_monto"},
		}
	}

	// Cabecera de tabla
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	pdf.SetTextColor(0, 0, 0)
	for _, c := range cols {
		pdf.CellFormat(c.Ancho, 8, c.Titulo, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Filas de datos
	pdf.SetFont("Arial", "", 9)
	for _, fila := range datos {
		for _, c := range cols {
			valor := fila[c.Clave]
			var txt string
			switch c.Clave {
			case "etiqueta":
				txt = utils.ToString(valor)
			case "pagos_count", "dev_count":
				txt = fmt.Sprintf("%d", utils.ToInt64(valor))
			default: // montos
				txt = fmt.Sprintf("%.2f", utils.ToFloat64(valor))
			}
			pdf.CellFormat(c.Ancho, 7, txt, "1", 0, "R", false, 0, "")
		}
		pdf.Ln(-1)
	}

	// Fila de totales
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(245, 245, 245)
	tot := calcularTotales(datos)
	for _, c := range cols {
		var txt string
		switch c.Clave {
		case "etiqueta":
			txt = "TOTALES"
		case "pagos_count":
			txt = fmt.Sprintf("%d", utils.ToInt64(tot["pagos_count"]))
		case "dev_count":
			txt = fmt.Sprintf("%d", utils.ToInt64(tot["dev_count"]))
		default: // montos
			txt = fmt.Sprintf("%.2f", utils.ToFloat64(tot[c.Clave]))
		}
		pdf.CellFormat(c.Ancho, 8, txt, "1", 0, "R", true, 0, "")
	}
	pdf.Ln(-1)

	// Enviar PDF directamente al ResponseWriter
	filename := fmt.Sprintf("reporte_%s_%s.pdf", tipo, time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	if err := pdf.Output(w); err != nil {
		utils.LogError("reporte_pdf_output")
	}
}
