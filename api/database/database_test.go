// Package database: tests de integracion con PostgreSQL real.
//
// Requiere PostgreSQL en 127.0.0.1:5432 (db=pasarela, user=postgres, password=1234).
// Si el servicio no esta disponible, los tests lo detectan y hacen t.Skip.
// No modifica codigo de produccion.
package database

import (
	"context"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Helper: DSN por defecto (espeja config defaults + entorno descrito)
// ---------------------------------------------------------------------------

const testDSN = "host=127.0.0.1 dbname=pasarela user=postgres password=1234 port=5432 sslmode=disable"

// pingPostgres verifica conectividad rapida (TCP dial, sin pgx).
func pingPostgres(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "127.0.0.1:5432", 2*time.Second)
	if err != nil {
		t.Skipf("PostgreSQL no disponible en 127.0.0.1:5432 — skip: %v", err)
	}
	conn.Close()
}

// initDB intenta Init y hace t.Skip si falla (servicio caido o DSN incorrecto).
func initDB(t *testing.T) *Database {
	t.Helper()
	pingPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := Init(ctx, testDSN, 2, 10)
	if err != nil {
		t.Skipf("No se pudo inicializar pool PostgreSQL — skip: %v", err)
	}
	t.Cleanup(func() { Close() })
	return db
}

// ===========================================================================
// Tests de Init
// ===========================================================================

func TestInit_DSNValido(t *testing.T) {
	pingPostgres(t)

	// Asegurar estado limpio
	Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Init(ctx, testDSN, 2, 10)
	if err != nil {
		t.Fatalf("Init con DSN valido fallo: %v", err)
	}
	if db == nil {
		t.Fatal("Init devolvio nil Database sin error")
	}
	// Limpiar
	Close()
}

func TestInit_DSNInvalido(t *testing.T) {
	Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := Init(ctx, "host=noexiste dbname=xxx user=x password=x port=9999", 1, 5)
	if err == nil {
		t.Fatal("Init con DSN invalido deberia devolver error")
	}
}

// ===========================================================================
// Tests de Get / Close
// ===========================================================================

func TestGet_DespuesDeInit(t *testing.T) {
	db := initDB(t)

	got := Get()
	if got == nil {
		t.Fatal("Get() devolvio nil despues de Init")
	}
	if got != db {
		t.Fatal("Get() devolvio instancia diferente a la de Init")
	}
}

func TestClose_NoPanic(t *testing.T) {
	initDB(t)
	Close() // no deberia panic
	if _db != nil {
		t.Fatal("Close() deberia dejar _db en nil")
	}
}

func TestGetPanicSinInit(t *testing.T) {
	Close()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Get() sin Init deberia panic")
		}
	}()
	_ = Get()
}

// ===========================================================================
// Tests de normalizeValue
// ===========================================================================

func TestNormalizeValue(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 30, 0, 0, time.UTC)

	cases := []struct {
		name string
		in   any
		want any
	}{
		{"nil se mantiene nil", nil, nil},

		// Numeric
		{"numeric valido -> float64", pgtype.Numeric{Int: big.NewInt(12345), Exp: -2, Valid: true}, 123.45},
		{"numeric invalido -> nil", pgtype.Numeric{Valid: false}, nil},

		// Enteros
		{"int2 valido -> int64", pgtype.Int2{Int16: 42, Valid: true}, int64(42)},
		{"int2 invalido -> nil", pgtype.Int2{Valid: false}, nil},
		{"int4 valido -> int64", pgtype.Int4{Int32: 4242, Valid: true}, int64(4242)},
		{"int4 invalido -> nil", pgtype.Int4{Valid: false}, nil},
		{"int8 valido -> int64", pgtype.Int8{Int64: 424242, Valid: true}, int64(424242)},
		{"int8 invalido -> nil", pgtype.Int8{Valid: false}, nil},

		// Flotantes
		{"float4 valido -> float64", pgtype.Float4{Float32: 1.5, Valid: true}, float64(1.5)},
		{"float4 invalido -> nil", pgtype.Float4{Valid: false}, nil},
		{"float8 valido -> float64", pgtype.Float8{Float64: 2.75, Valid: true}, 2.75},
		{"float8 invalido -> nil", pgtype.Float8{Valid: false}, nil},

		// Texto
		{"text valido -> string", pgtype.Text{String: "hola", Valid: true}, "hola"},
		{"text invalido -> nil", pgtype.Text{Valid: false}, nil},

		// Bool
		{"bool valido -> bool", pgtype.Bool{Bool: true, Valid: true}, true},
		{"bool invalido -> nil", pgtype.Bool{Valid: false}, nil},

		// Timestamps
		{"timestamp valido -> time.Time", pgtype.Timestamp{Time: now, Valid: true}, now},
		{"timestamp invalido -> nil", pgtype.Timestamp{Valid: false}, nil},
		{"timestamptz valido -> time.Time", pgtype.Timestamptz{Time: now, Valid: true}, now},
		{"timestamptz invalido -> nil", pgtype.Timestamptz{Valid: false}, nil},
		{"date valido -> time.Time", pgtype.Date{Time: now, Valid: true}, now},
		{"date invalido -> nil", pgtype.Date{Valid: false}, nil},

		// UUID
		{"uuid valido -> string", pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}, "01020304-0506-0708-090a-0b0c0d0e0f10"},
		{"uuid invalido -> nil", pgtype.UUID{Valid: false}, nil},

		// []byte -> string
		{"[]byte -> string", []byte("bytes"), "bytes"},

		// Tipos Go planos pasan tal cual
		{"int plano se mantiene", 7, 7},
		{"string plana se mantiene", "x", "x"},
		{"float64 plano se mantiene", 3.14, 3.14},
		{"bool plano se mantiene", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeValue(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeValue(%#v) = %#v, quiere %#v", tc.in, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// Tests de rowToMap (con PostgreSQL real)
// ===========================================================================

func TestRowToMap_SelectReal(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	rows, err := db.Select(ctx, "SELECT 1::int4 AS n, 'hola'::text AS s, true::bool AS b")
	if err != nil {
		t.Fatalf("Select fallo: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("esperaba 1 fila, obtuve %d", len(rows))
	}
	row := rows[0]

	// pgx devuelve int32 para int4, no pasa por pgtype.Int4 → normalizeValue lo deja tal cual.
	n := row["n"]
	if n != int32(1) && n != int64(1) {
		t.Errorf("n = %v (tipo %T), quiero int32(1) o int64(1)", n, n)
	}
	if s, ok := row["s"].(string); !ok || s != "hola" {
		t.Errorf("s = %v (tipo %T), quiere string \"hola\"", row["s"], row["s"])
	}
	if b, ok := row["b"].(bool); !ok || b != true {
		t.Errorf("b = %v (tipo %T), quiere bool(true)", row["b"], row["b"])
	}
}

func TestRowToMap_MultiplesFilas(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	rows, err := db.Select(ctx, "SELECT 1::int4 AS n UNION ALL SELECT 2::int4 AS n ORDER BY n")
	if err != nil {
		t.Fatalf("Select fallo: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("esperaba 2 filas, obtuve %d", len(rows))
	}
	expects := []int32{1, 2}
	for i, want := range expects {
		n := rows[i]["n"]
		// pgx puede devolver int32 o int64 segun la version del driver
		if n != int32(want) && n != int64(want) {
			t.Errorf("fila %d: n = %v (tipo %T), quiero %d", i, n, n, want)
		}
	}
}

// ===========================================================================
// Tests de Select / SelectOne / SelectVal (no destructivos)
// ===========================================================================

func TestSelect_UnionQuery(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	rows, err := db.Select(ctx, "SELECT 1::int4 AS a, 2::int4 AS b")
	if err != nil {
		t.Fatalf("Select fallo: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("esperaba 1 fila, obtuve %d", len(rows))
	}
	a := rows[0]["a"]
	b := rows[0]["b"]
	if (a != int32(1) && a != int64(1)) || (b != int32(2) && b != int64(2)) {
		t.Errorf("valores inesperados: a=%v b=%v", a, b)
	}
}

func TestSelectOne(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	row, err := db.SelectOne(ctx, "SELECT 42::int4 AS answer, 'universe'::text AS label")
	if err != nil {
		t.Fatalf("SelectOne fallo: %v", err)
	}
	if row == nil {
		t.Fatal("SelectOne devolvio nil")
	}
	a := row["answer"]
	if a != int32(42) && a != int64(42) {
		t.Errorf("answer = %v (tipo %T), quiero int32/int64(42)", a, a)
	}
	if row["label"] != "universe" {
		t.Errorf("label = %v, quiero \"universe\"", row["label"])
	}
}

func TestSelectOne_SinFilas(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	row, err := db.SelectOne(ctx, "SELECT 1 WHERE false")
	if err != nil {
		t.Fatalf("SelectOne fallo: %v", err)
	}
	if row != nil {
		t.Errorf("SelectOne sin filas deberia devolver nil, obtuve %v", row)
	}
}

func TestSelectVal(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	v, err := db.SelectVal(ctx, "SELECT current_database()")
	if err != nil {
		t.Fatalf("SelectVal fallo: %v", err)
	}
	if v != "pasarela" {
		t.Errorf("current_database() = %v, quiero \"pasarela\"", v)
	}
}

func TestSelectVal_Entero(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	v, err := db.SelectVal(ctx, "SELECT 99::int4")
	if err != nil {
		t.Fatalf("SelectVal fallo: %v", err)
	}
	// pgx puede devolver int32 para int4 — normalizeValue no lo convierte a int64
	// porque no pasa por pgtype.Int4
	if v != int32(99) && v != int64(99) {
		t.Errorf("SELECT 99 = %v (tipo %T), quiero int32/int64(99)", v, v)
	}
}

func TestSelectVal_SinFilas(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	v, err := db.SelectVal(ctx, "SELECT 1 WHERE false")
	if err != nil {
		t.Fatalf("SelectVal fallo: %v", err)
	}
	if v != nil {
		t.Errorf("SelectVal sin filas deberia devolver nil, obtuvo %v", v)
	}
}

// ===========================================================================
// Tests de Execute (no destructivo — usa DROP IF EXISTS)
// ===========================================================================

func TestExecute_DDL(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	// Crear y eliminar una tabla temporal de prueba
	tag, err := db.Execute(ctx, `CREATE TEMPORARY TABLE test_ddl_aux (id int)`)
	if err != nil {
		t.Fatalf("CREATE TABLE fallo: %v", err)
	}
	t.Logf("Execute CREATE tag: %s", tag)

	tag, err = db.Execute(ctx, `DROP TABLE test_ddl_aux`)
	if err != nil {
		t.Fatalf("DROP TABLE fallo: %v", err)
	}
	t.Logf("Execute DROP tag: %s", tag)
}

// ===========================================================================
// Tests de InsertReturning (con tabla temporal, sin contaminar)
// ===========================================================================

func TestInsertReturning(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	// Crear tabla temporal con serial
	_, err := db.Execute(ctx, `CREATE TEMPORARY TABLE test_insert_ret (id serial PRIMARY KEY, val text)`)
	if err != nil {
		t.Fatalf("CREATE TABLE fallo: %v", err)
	}

	v, err := db.InsertReturning(ctx,
		`INSERT INTO test_insert_ret (val) VALUES ($1) RETURNING id`, "hello")
	if err != nil {
		t.Fatalf("InsertReturning fallo: %v", err)
	}
	if v == nil {
		t.Fatal("InsertReturning devolvio nil")
	}
	// serial puede devolver int32 o int64
	var id int64
	switch vid := v.(type) {
	case int32:
		id = int64(vid)
	case int64:
		id = vid
	default:
		t.Fatalf("InsertReturning tipo inesperado: %T (%v)", v, v)
	}
	if id < 1 {
		t.Errorf("id deberia ser >= 1, obtuvo %d", id)
	}
	t.Logf("InsertReturning id = %d", id)

	// Verificar que el dato se puede leer
	row, err := db.SelectOne(ctx, `SELECT val FROM test_insert_ret WHERE id = $1`, id)
	if err != nil {
		t.Fatalf("SelectOne fallo: %v", err)
	}
	if row == nil || row["val"] != "hello" {
		t.Errorf("dato insertado no encontrado o incorrecto: %v", row)
	}
}

// ===========================================================================
// Tests de InsertMany (batch insert con tabla temporal)
// ===========================================================================

func TestInsertMany(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	_, err := db.Execute(ctx, `CREATE TEMPORARY TABLE test_insert_many (id serial PRIMARY KEY, n int)`)
	if err != nil {
		t.Fatalf("CREATE TABLE fallo: %v", err)
	}

	paramsList := [][]any{
		{10},
		{20},
		{30},
	}
	n, err := db.InsertMany(ctx,
		`INSERT INTO test_insert_many (n) VALUES ($1)`, paramsList)
	if err != nil {
		t.Fatalf("InsertMany fallo: %v", err)
	}
	if n != 3 {
		t.Errorf("InsertMany devolvio %d, quiero 3", n)
	}

	// Verificar cantidad — count(*) puede devolver int64 o int32
	v, err := db.SelectVal(ctx, `SELECT count(*)::int8 FROM test_insert_many`)
	if err != nil {
		t.Fatalf("SelectVal fallo: %v", err)
	}
	if v != int64(3) && v != int32(3) {
		t.Errorf("count = %v, quiero 3", v)
	}
}

func TestInsertMany_Vacio(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	n, err := db.InsertMany(ctx, `INSERT INTO test_insert_many DEFAULT VALUES`, [][]any{})
	if err != nil {
		t.Fatalf("InsertMany vacio fallo: %v", err)
	}
	if n != 0 {
		t.Errorf("InsertMany vacio devolvio %d, quiero 0", n)
	}
}

// ===========================================================================
// Tests de GetTables / GetTableColumns (schema queries)
// ===========================================================================

func TestGetTables(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	tables, err := db.GetTables(ctx)
	if err != nil {
		t.Fatalf("GetTables fallo: %v", err)
	}
	// No asumimos tablas especificas; solo que no fallo
	t.Logf("GetTables: %v (%d tablas)", tables, len(tables))
}

func TestGetTableColumns_TablaPublica(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	// Buscar una tabla del schema public que exista en la DB pasarela
	tables, err := db.GetTables(ctx)
	if err != nil || len(tables) == 0 {
		t.Skip("No hay tablas en schema public — skip")
	}
	target := tables[0]

	cols, err := db.GetTableColumns(ctx, target)
	if err != nil {
		t.Fatalf("GetTableColumns(%s) fallo: %v", target, err)
	}
	if len(cols) == 0 {
		t.Errorf("GetTableColumns(%s) devolvio 0 columnas", target)
	}
	t.Logf("%s tiene %d columnas", target, len(cols))

	for _, c := range cols {
		if _, ok := c["column_name"]; !ok {
			t.Errorf("columna sin 'column_name': %v", c)
		}
	}
}

func TestGetTableColumns_TablaInexistente(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	cols, err := db.GetTableColumns(ctx, "tabla_que_no_existe_xyz")
	if err != nil {
		t.Fatalf("GetTableColumns con tabla inexistente fallo: %v", err)
	}
	if len(cols) != 0 {
		t.Errorf("esperaba 0 columnas, obtuve %d", len(cols))
	}
}

func TestGetTablePrimaryKey(t *testing.T) {
	db := initDB(t)
	ctx := context.Background()

	// pg_class tiene un indice unico en oid pero no tiene PK formal;
	// probamos con una tabla que sepamos tiene PK si existe,
	// o simplemente verificamos que no fallo.
	pk, err := db.GetTablePrimaryKey(ctx, "pg_class")
	if err != nil {
		t.Fatalf("GetTablePrimaryKey fallo: %v", err)
	}
	t.Logf("GetTablePrimaryKey(pg_class) = %q (puede ser vacio si no tiene PK)", pk)
}
