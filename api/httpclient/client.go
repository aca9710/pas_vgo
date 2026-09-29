// Package httpclient replica client.py: un unico http.Client compartido
// (timeout 10s, verify=False como httpx).
package httpclient

import (
	"crypto/tls"
	"net/http"
	"time"
)

var _client *http.Client

// Get retorna la instancia global del http.Client.
func Get() *http.Client {
	if _client == nil {
		panic("http client not initialized. Call Init() from main first.")
	}
	return _client
}

// Init crea el http.Client global con timeout y TLS insecure
// (equivalente a httpx.AsyncClient(timeout=timeout, verify=False)).
func Init(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // verify=False como httpx
	}
	_client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	return _client
}

// Close cierra el transporte del cliente global.
func Close() {
	if _client != nil {
		if tr, ok := _client.Transport.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
		_client = nil
	}
}