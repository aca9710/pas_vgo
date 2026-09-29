package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashPassword genera el hash SHA256 hex de una contraseña.
// Equivalente a hashlib.sha256(...).hexdigest() de Python.
func HashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}