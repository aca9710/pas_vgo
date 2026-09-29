// Package redisclient: tests de integracion con Redis real.
//
// Si Redis no esta disponible en localhost:6379, los tests que requieren
// conexion hacen t.Skip (no fallan). El test de DSN invalido NO requiere
// conexion y siempre corre.
package redisclient

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helper: verificar disponibilidad de Redis
// ---------------------------------------------------------------------------

func redisAvailable(t *testing.T) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func skipIfNoRedis(t *testing.T) {
	t.Helper()
	if !redisAvailable(t) {
		t.Skip("Redis no disponible en 127.0.0.1:6379 — skip")
	}
}

const testRedisDSN = "redis://localhost:6379/0"

// initRedis intenta Init; si Redis no esta, t.Skip.
func initRedis(t *testing.T) {
	t.Helper()
	skipIfNoRedis(t)
	Close()
	_, err := Init(testRedisDSN)
	if err != nil {
		t.Skipf("No se pudo inicializar Redis — skip: %v", err)
	}
	t.Cleanup(func() { Close() })
}

// ===========================================================================
// Tests de Init
// ===========================================================================

func TestInit_DSNInvalido(t *testing.T) {
	Close()

	// "no-es-url" no es una URL valida — ParseURL falla SIN intentar conexion.
	_, err := Init("no-es-url")
	if err == nil {
		t.Fatal("Init con DSN invalido deberia devolver error")
	}
	if _client != nil {
		t.Fatal("Init fallido no deberia dejar _client asignado")
	}
}

func TestInit_DSNValido_PingOK(t *testing.T) {
	skipIfNoRedis(t)
	Close()

	client, err := Init(testRedisDSN)
	if err != nil {
		t.Fatalf("Init con DSN valido fallo: %v", err)
	}
	if client == nil {
		t.Fatal("Init devolvio nil client sin error")
	}
	Close()
}

// ===========================================================================
// Tests de Get / Close
// ===========================================================================

func TestGet_DespuesDeInit(t *testing.T) {
	initRedis(t)

	got := Get()
	if got == nil {
		t.Fatal("Get() devolvio nil despues de Init")
	}
}

func TestClose_NoPanic(t *testing.T) {
	initRedis(t)
	Close()
	if _client != nil {
		t.Fatal("Close() deberia dejar _client en nil")
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

func TestCloseSinInit(t *testing.T) {
	Close()
	if _client != nil {
		t.Fatal("Close() con _client nil no deberia cambiar nada")
	}
}

// ===========================================================================
// Tests de SET/GET/DEL roundtrip (requieren Redis vivo)
// ===========================================================================

func TestSETGET_Roundtrip(t *testing.T) {
	initRedis(t)

	ctx := context.Background()
	key := fmt.Sprintf("test:go:%d", time.Now().UnixNano())
	val := "integration-test-value"

	// SET
	if err := Get().Set(ctx, key, val, 30*time.Second).Err(); err != nil {
		t.Fatalf("SET fallo: %v", err)
	}

	// GET
	got, err := Get().Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("GET fallo: %v", err)
	}
	if got != val {
		t.Errorf("GET = %q, quiero %q", got, val)
	}

	// DEL
	n, err := Get().Del(ctx, key).Result()
	if err != nil {
		t.Fatalf("DEL fallo: %v", err)
	}
	if n != 1 {
		t.Errorf("DEL devolvio %d, quiero 1", n)
	}

	// Verificar que ya no existe
	_, err = Get().Get(ctx, key).Result()
	if err == nil {
		t.Error("GET despues de DEL deberia devolver error")
	}
}

func TestSETGET_KeyNumerica(t *testing.T) {
	initRedis(t)

	ctx := context.Background()
	key := fmt.Sprintf("test:go:num:%d", time.Now().UnixNano())

	Get().Set(ctx, key, 42, 30*time.Second)
	defer Get().Del(ctx, key)

	got, err := Get().Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("GET fallo: %v", err)
	}
	if got != "42" {
		t.Errorf("GET = %q, quiero \"42\"", got)
	}
}
