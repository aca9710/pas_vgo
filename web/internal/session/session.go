package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"webgo/internal/db"
)

// NewSessionID genera un UUID v4 aleatorio (equivalente a str(uuid.uuid4()))
func NewSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return hex.EncodeToString(b[0:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:16])
}

// CreateAdminSession crea una sesión de admin con expiración (DB-backed)
func CreateAdminSession(ctx context.Context, usuarioID int64, expiresIn time.Duration) (string, error) {
	sessionID := NewSessionID()
	expires := time.Now().Add(expiresIn)
	_, err := db.Instance.Exec(ctx,
		"INSERT INTO admin_sesion (id, usuario_id, expires_at) VALUES ($1, $2, $3)",
		sessionID, usuarioID, expires)
	return sessionID, err
}

// DeleteAdminSession elimina una sesión de admin
func DeleteAdminSession(ctx context.Context, sessionID string) error {
	_, err := db.Instance.Exec(ctx, "DELETE FROM admin_sesion WHERE id = $1", sessionID)
	return err
}

// CreateClienteSession crea una sesión de cliente con expiración (DB-backed)
func CreateClienteSession(ctx context.Context, clienteID int64, expiresIn time.Duration) (string, error) {
	sessionID := NewSessionID()
	expires := time.Now().Add(expiresIn)
	_, err := db.Instance.Exec(ctx,
		"INSERT INTO sesion_cliente (id, cliente_id, expires_at) VALUES ($1, $2, $3)",
		sessionID, clienteID, expires)
	return sessionID, err
}

// DeleteClienteSession elimina una sesión de cliente
func DeleteClienteSession(ctx context.Context, sessionID string) error {
	_, err := db.Instance.Exec(ctx, "DELETE FROM sesion_cliente WHERE id = $1", sessionID)
	return err
}