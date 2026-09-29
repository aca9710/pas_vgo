package utils

import "fmt"

// Helpers de conversión de tipos para valores de pgx RowToMap.
// pgx v5 mapea int4 -> int32, int8 -> int64, numeric -> float64, etc.

// ToInt64 convierte cualquier valor numérico a int64 (0 si no es numérico).
func ToInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int16:
		return int64(n)
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	}
	return 0
}

// ToFloat64 convierte cualquier valor numérico a float64.
func ToFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case int16:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

// ToString convierte un valor a string ("" si es nil).
func ToString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// ToBool convierte un valor a bool.
func ToBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// ToInt convierte un valor a int.
func ToInt(v any) int {
	return int(ToInt64(v))
}

// ToStrPtr convierte un valor a *string (nil si es nil o vacío).
func ToStrPtr(v any) *string {
	if v == nil {
		return nil
	}
	s := ToString(v)
	if s == "" {
		return nil
	}
	return &s
}