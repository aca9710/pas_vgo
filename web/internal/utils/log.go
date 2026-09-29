package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

// LogError registra un error en log/error_YYYYMMDD.log
// Equivalente a ver_error() de Python. USO OBLIGATORIO en todo
// bloque except de operaciones de BD o código que pueda fallar.
func LogError(contexto string) {
	dir := "log"
	os.MkdirAll(dir, 0o755)
	fecha := time.Now()
	fname := filepath.Join(dir, fmt.Sprintf("error_%s.log", fecha.Format("20060102")))

	f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Println("ERROR GRAVE: no se puede abrir el fichero de errores:", err)
		return
	}
	defer f.Close()

	sep := "------------------------------------------------------------------------------"
	fmt.Fprintf(f, "%s\n%s, %s \n", sep, fecha.Format("02/01/2006"), fecha.Format("03:04:05 PM"))
	if contexto != "" {
		fmt.Fprintf(f, "Contexto: %s\n", contexto)
	}
	f.Write(debug.Stack())
	f.WriteString("\n")
}

// Traza registra una traza de operación en log/traza_YYYYMMDD.log
func Traza(texto string) {
	dir := "log"
	os.MkdirAll(dir, 0o755)
	fecha := time.Now()
	fname := filepath.Join(dir, fmt.Sprintf("traza_%s.log", fecha.Format("20060102")))

	f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "%s, %s %s\n", fecha.Format("02/01/2006"), fecha.Format("03:04:05 PM"), texto)
}