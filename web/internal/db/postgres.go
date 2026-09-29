package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Instance es el pool global de conexiones (equivalente a `db` de Python)
var Instance *DB

// DB envuelve el pool de conexiones pgx
type DB struct {
	Pool *pgxpool.Pool
}

// Connect crea el pool de conexiones a PostgreSQL leyendo .env / variables de entorno.
// Mismos defaults que database.py de Python.
func Connect() error {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "1234"
	}
	name := os.Getenv("DB_NAME")
	if name == "" {
		name = "pasarela"
	}

	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, name)

	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return fmt.Errorf("error parseando config de BD: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 5

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("error creando pool: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return fmt.Errorf("error conectando a BD: %w", err)
	}

	Instance = &DB{Pool: pool}
	fmt.Println("✅ Conexión a base de datos establecida")
	return nil
}

// Close cierra el pool de conexiones
func (d *DB) Close() {
	if d.Pool != nil {
		d.Pool.Close()
		fmt.Println("🔒 Conexión a base de datos cerrada")
	}
}

// Exec ejecuta una query sin retornar filas
func (d *DB) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return d.Pool.Exec(ctx, query, args...)
}

// Query ejecuta una query y retorna filas
func (d *DB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return d.Pool.Query(ctx, query, args...)
}

// QueryRow ejecuta una query y retorna una fila
func (d *DB) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return d.Pool.QueryRow(ctx, query, args...)
}

// Fetch retorna todas las filas como mapas (equivalente a db.fetch de Python)
func (d *DB) Fetch(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := d.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToMap)
}

// FetchOne retorna una fila como mapa o nil (equivalente a db.fetch_one)
func (d *DB) FetchOne(ctx context.Context, query string, args ...any) (map[string]any, error) {
	rows, err := d.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m, err := pgx.CollectOneRow(rows, pgx.RowToMap)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// FetchVal retorna un solo valor (equivalente a db.fetch_val)
func (d *DB) FetchVal(ctx context.Context, query string, args ...any) (any, error) {
	var val any
	err := d.Pool.QueryRow(ctx, query, args...).Scan(&val)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Collect retorna filas tipadas por nombre de columna (requiere tags `db:` en el struct)
func Collect[T any](ctx context.Context, query string, args ...any) ([]T, error) {
	rows, err := Instance.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[T])
}

// CollectOne retorna una fila tipada o nil
func CollectOne[T any](ctx context.Context, query string, args ...any) (*T, error) {
	rows, err := Instance.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[T])
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}