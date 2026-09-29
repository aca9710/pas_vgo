package db

import (
	"context"
	"fmt"

	"webgo/internal/utils"
)

// Migrate crea todas las tablas si no existen y el admin inicial.
// Equivalente a los init_*_tables + crear_admin_inicial de database.py.
func Migrate(ctx context.Context) error {
	if err := initClienteTables(ctx); err != nil {
		return err
	}
	if err := initNotificacionesTables(ctx); err != nil {
		return err
	}
	if err := initEntidadTables(ctx); err != nil {
		return err
	}
	if err := initAdminTables(ctx); err != nil {
		return err
	}
	if err := migrateEntidadColumns(ctx); err != nil {
		return err
	}
	if err := initTiendaTables(ctx); err != nil {
		return err
	}
	if err := initPagoTables(ctx); err != nil {
		return err
	}
	if err := crearAdminInicial(ctx); err != nil {
		return err
	}
	return nil
}

func initClienteTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS cliente (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			nombre VARCHAR(255) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			activo BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sesion_cliente (
			id VARCHAR(36) PRIMARY KEY,
			cliente_id INTEGER REFERENCES cliente(id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sesion_cliente_cliente ON sesion_cliente(cliente_id)`,
	}
	for _, q := range queries {
		if _, err := Instance.Exec(ctx, q); err != nil {
			utils.LogError("init_cliente_tables")
			return err
		}
	}
	fmt.Println("✅ Tablas de cliente inicializadas")
	return nil
}

func initNotificacionesTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS notificacion_pago (
			id SERIAL PRIMARY KEY,
			phone VARCHAR(20) NOT NULL,
			external_id VARCHAR(50) NOT NULL,
			tmid VARCHAR(50),
			bankid VARCHAR(50),
			status VARCHAR(20) NOT NULL,
			source VARCHAR(50),
			msg TEXT,
			bank VARCHAR(100),
			fecha TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS notificacion_devolucion (
			id SERIAL PRIMARY KEY,
			refund_id VARCHAR(50) NOT NULL,
			reference_refund VARCHAR(50),
			reference_refund_tm VARCHAR(50),
			success BOOLEAN DEFAULT FALSE,
			resultmsg TEXT,
			status VARCHAR(20),
			external_id VARCHAR(50),
			bankid VARCHAR(50),
			tmid VARCHAR(50),
			fecha TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := Instance.Exec(ctx, q); err != nil {
			utils.LogError("init_notificaciones_tables")
			return err
		}
	}
	fmt.Println("✅ Tablas de notificaciones inicializadas")
	return nil
}

func initEntidadTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS entidad (
			id SERIAL PRIMARY KEY,
			nombre VARCHAR(255) NOT NULL,
			llave VARCHAR(100) UNIQUE NOT NULL,
			admin_id INTEGER REFERENCES admin_usuario(id) ON DELETE SET NULL,
			activo BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entidad_llave ON entidad(llave)`,
	}
	for _, q := range queries {
		if _, err := Instance.Exec(ctx, q); err != nil {
			utils.LogError("init_entidad_tables")
			return err
		}
	}
	fmt.Println("✅ Tabla de entidades inicializada")
	return nil
}

func initAdminTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS admin_usuario (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password_hash VARCHAR(64) NOT NULL,
			rol VARCHAR(20) NOT NULL DEFAULT 'user',
			activo BOOLEAN DEFAULT TRUE,
			entidad_id INTEGER REFERENCES entidad(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS admin_usuario_tienda (
			usuario_id INTEGER REFERENCES admin_usuario(id) ON DELETE CASCADE,
			tienda_uid VARCHAR(20) REFERENCES tiendas(uid) ON DELETE CASCADE,
			PRIMARY KEY (usuario_id, tienda_uid)
		)`,
		`CREATE TABLE IF NOT EXISTS admin_sesion (
			id VARCHAR(36) PRIMARY KEY,
			usuario_id INTEGER REFERENCES admin_usuario(id) ON DELETE CASCADE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_usuario_tienda_usuario ON admin_usuario_tienda(usuario_id)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_sesion_usuario ON admin_sesion(usuario_id)`,
	}
	for _, q := range queries {
		if _, err := Instance.Exec(ctx, q); err != nil {
			utils.LogError("init_admin_tables")
			return err
		}
	}
	fmt.Println("✅ Tablas de admin inicializadas")
	return nil
}

func migrateEntidadColumns(ctx context.Context) error {
	queries := []string{
		`ALTER TABLE tiendas ADD COLUMN IF NOT EXISTS entidad_id INTEGER REFERENCES entidad(id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_tiendas_entidad ON tiendas(entidad_id)`,
		`ALTER TABLE admin_usuario ADD COLUMN IF NOT EXISTS entidad_id INTEGER REFERENCES entidad(id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_admin_usuario_entidad ON admin_usuario(entidad_id)`,
	}
	for _, q := range queries {
		if _, err := Instance.Exec(ctx, q); err != nil {
			utils.LogError("migrate_entidad_columns")
			return err
		}
	}
	fmt.Println("✅ Columnas entidad_id migradas")
	return nil
}

func initTiendaTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tiendas (
			uid VARCHAR(20) PRIMARY KEY,
			nombre VARCHAR(255),
			produccion VARCHAR(500),
			desarrollo VARCHAR(500),
			activa BOOLEAN DEFAULT TRUE,
			entidad_id INTEGER REFERENCES entidad(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tpv (
			id SERIAL PRIMARY KEY,
			uid VARCHAR(20) REFERENCES tiendas(uid) ON DELETE CASCADE,
			dirip VARCHAR(45) NOT NULL,
			identoptima VARCHAR(100),
			nombre VARCHAR(255),
			activo BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tiendas_entidad ON tiendas(entidad_id)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_usuario_entidad ON admin_usuario(entidad_id)`,
	}
	for _, q := range queries {
		if _, err := Instance.Exec(ctx, q); err != nil {
			utils.LogError("init_tienda_tables")
			return err
		}
	}
	fmt.Println("✅ Tablas de tiendas inicializadas")
	return nil
}

func initPagoTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS pagos (
			id SERIAL PRIMARY KEY,
			idoperacion VARCHAR(50) UNIQUE NOT NULL,
			uid VARCHAR(20) REFERENCES tiendas(uid) ON DELETE SET NULL,
			cliente VARCHAR(255) NOT NULL,
			importe DECIMAL(12,2) NOT NULL,
			estado INTEGER DEFAULT 0,
			fecha TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			orderid INTEGER,
			msg INTEGER,
			tmid VARCHAR(50),
			bankid VARCHAR(50),
			url INTEGER REFERENCES url(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS devolucion (
			id SERIAL PRIMARY KEY,
			idoperacion VARCHAR(50),
			uid VARCHAR(20) REFERENCES tiendas(uid) ON DELETE SET NULL,
			cliente VARCHAR(255) NOT NULL,
			importe DECIMAL(12,2) NOT NULL,
			estado INTEGER DEFAULT 0,
			fecha TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			external_id VARCHAR(50),
			resultmsg TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS url (
			id SERIAL PRIMARY KEY,
			url VARCHAR(500) NOT NULL,
			activo BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS msg (
			id SERIAL PRIMARY KEY,
			texto TEXT NOT NULL,
			tipo VARCHAR(20) DEFAULT 'info',
			activo BOOLEAN DEFAULT TRUE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pagos_uid ON pagos(uid)`,
		`CREATE INDEX IF NOT EXISTS idx_pagos_cliente ON pagos(cliente)`,
		`CREATE INDEX IF NOT EXISTS idx_pagos_fecha ON pagos(fecha)`,
		`CREATE INDEX IF NOT EXISTS idx_devolucion_cliente ON devolucion(cliente)`,
	}
	for _, q := range queries {
		if _, err := Instance.Exec(ctx, q); err != nil {
			utils.LogError("init_pago_tables")
			return err
		}
	}
	fmt.Println("✅ Tablas de pagos inicializadas")
	return nil
}

// crearAdminInicial crea el usuario admin/admin si no existe
func crearAdminInicial(ctx context.Context) error {
	existe, err := Instance.FetchOne(ctx, "SELECT id FROM admin_usuario WHERE username = 'admin'")
	if err != nil {
		utils.LogError("crear_admin_inicial")
		return err
	}
	if existe == nil {
		adminHash := utils.HashPassword("admin")
		_, err := Instance.Exec(ctx,
			"INSERT INTO admin_usuario (username, password_hash, rol, activo) VALUES ('admin', $1, 'admin', TRUE)",
			adminHash)
		if err != nil {
			utils.LogError("crear_admin_inicial")
			return err
		}
		fmt.Println("✅ Usuario admin inicial creado (admin/admin)")
	}
	return nil
}