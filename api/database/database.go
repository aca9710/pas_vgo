// Package database replica database.py: acceso a PostgreSQL con pgxpool.
// Todas las queries usan placeholders $1, $2, ... parameterizados.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// =============================================================================
// Database — CRUD con queries parameterizadas
// =============================================================================

type Database struct {
	pool *pgxpool.Pool
}

// normalizeValue convierte tipos pgx a tipos Go planos (como hace asyncpg:
// numeric -> float, timestamp -> time.Time, etc.).
//
// OJO: al escanear en *any, pgx v5 devuelve el tipo NATIVO de Go (int32 para
// int4, int64 para int8, float64 para numeric, string para varchar, bool, ...).
// Los casos pgtype.* de abajo solo aplican si algun dia se escanea en un
// pgtype.X concreto; con el codigo actual son defensivos. Los consumidores
// (routers.toInt, utils.toInt, ...) deben reconocer los tipos nativos: un
// int4 de PostgreSQL llega como int32, no como int ni como int64.
func normalizeValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case pgtype.Numeric:
		f, err := t.Float64Value()
		if err != nil || !f.Valid {
			return nil
		}
		return f.Float64
	case pgtype.Int2:
		if t.Valid {
			return int64(t.Int16)
		}
		return nil
	case pgtype.Int4:
		if t.Valid {
			return int64(t.Int32)
		}
		return nil
	case pgtype.Int8:
		if t.Valid {
			return t.Int64
		}
		return nil
	case pgtype.Float4:
		if t.Valid {
			return float64(t.Float32)
		}
		return nil
	case pgtype.Float8:
		if t.Valid {
			return t.Float64
		}
		return nil
	case pgtype.Text:
		if t.Valid {
			return t.String
		}
		return nil
	case pgtype.Bool:
		if t.Valid {
			return t.Bool
		}
		return nil
	case pgtype.Timestamp:
		if t.Valid {
			return t.Time
		}
		return nil
	case pgtype.Timestamptz:
		if t.Valid {
			return t.Time
		}
		return nil
	case pgtype.Date:
		if t.Valid {
			return t.Time
		}
		return nil
	case pgtype.UUID:
		if t.Valid {
			return t.String()
		}
		return nil
	case []byte:
		return string(t)
	default:
		return v
	}
}

// rowToMap convierte una fila pgx a map[string]any con valores normalizados.
func rowToMap(rows pgx.Rows) (map[string]any, error) {
	fields := rows.FieldDescriptions()
	values := make([]any, len(fields))
	ptrs := make([]any, len(fields))
	for i := range fields {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(fields))
	for i, f := range fields {
		out[string(f.Name)] = normalizeValue(values[i])
	}
	return out, nil
}

// Select ejecuta un SELECT y retorna lista de mapas.
func (d *Database) Select(ctx context.Context, query string, params ...any) ([]map[string]any, error) {
	rows, err := d.pool.Query(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]any
	for rows.Next() {
		m, err := rowToMap(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// SelectOne ejecuta un SELECT que espera un unico resultado o nil.
func (d *Database) SelectOne(ctx context.Context, query string, params ...any) (map[string]any, error) {
	rows, err := d.pool.Query(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		// No rows — check for iteration error.
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	m, err := rowToMap(rows)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// SelectVal ejecuta un SELECT que retorna un unico valor escalar.
func (d *Database) SelectVal(ctx context.Context, query string, params ...any) (any, error) {
	rows, err := d.pool.Query(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	fields := rows.FieldDescriptions()
	values := make([]any, len(fields))
	ptrs := make([]any, len(fields))
	for i := range fields {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}
	return normalizeValue(values[0]), nil
}

// Execute ejecuta INSERT/UPDATE/DELETE y retorna el command tag.
func (d *Database) Execute(ctx context.Context, query string, params ...any) (string, error) {
	tag, err := d.pool.Exec(ctx, query, params...)
	if err != nil {
		return "", err
	}
	return tag.String(), nil
}

// InsertReturning ejecuta INSERT ... RETURNING y retorna el primer valor.
func (d *Database) InsertReturning(ctx context.Context, query string, params ...any) (any, error) {
	var v any
	err := d.pool.QueryRow(ctx, query, params...).Scan(&v)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return normalizeValue(v), nil
}

// InsertMany ejecuta un batch insert.
func (d *Database) InsertMany(ctx context.Context, query string, paramsList [][]any) (int, error) {
	if len(paramsList) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, p := range paramsList {
		batch.Queue(query, p...)
	}
	br := d.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range paramsList {
		if _, err := br.Exec(); err != nil {
			return 0, err
		}
	}
	return len(paramsList), nil
}

// =============================================================================
// UTILIDADES DE ESQUEMA
// =============================================================================

// GetTables lista las tablas del esquema public.
func (d *Database) GetTables(ctx context.Context) ([]string, error) {
	query := `
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = 'public'
        ORDER BY table_name`
	rows, err := d.Select(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if s, ok := r["table_name"].(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// GetTableColumns obtiene informacion de columnas de una tabla.
func (d *Database) GetTableColumns(ctx context.Context, tableName string) ([]map[string]any, error) {
	query := `
        SELECT
            column_name,
            data_type,
            character_maximum_length,
            numeric_precision,
            numeric_scale,
            is_nullable,
            column_default
        FROM information_schema.columns
        WHERE table_name = $1 AND table_schema = 'public'
        ORDER BY ordinal_position`
	return d.Select(ctx, query, tableName)
}

// GetTablePrimaryKey obtiene el nombre de la columna PK de una tabla.
func (d *Database) GetTablePrimaryKey(ctx context.Context, tableName string) (string, error) {
	query := `
        SELECT kcu.column_name
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
            ON tc.constraint_name = kcu.constraint_name
        WHERE tc.constraint_type = 'PRIMARY KEY'
            AND tc.table_name = $1`
	m, err := d.SelectOne(ctx, query, tableName)
	if err != nil {
		return "", err
	}
	if m == nil {
		return "", nil
	}
	s, _ := m["column_name"].(string)
	return s, nil
}

// =============================================================================
// GLOBAL STATE (gestionado desde main.go)
// =============================================================================

var _db *Database

// Get retorna la instancia global de Database.
func Get() *Database {
	if _db == nil {
		panic("Database not initialized. Call Init() from main first.")
	}
	return _db
}

// Init crea el pool pgxpool y la instancia global de Database.
func Init(ctx context.Context, dsn string, minConns, maxConns int) (*Database, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MinConns = int32(minConns)
	cfg.MaxConns = int32(maxConns)
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	_db = &Database{pool: pool}
	return _db, nil
}

// Close cierra el pool global.
func Close() {
	if _db != nil && _db.pool != nil {
		_db.pool.Close()
		_db = nil
	}
}