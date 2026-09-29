-- =====================================================
-- Script de datos de prueba - Sistema Admin
-- =====================================================

-- --------------------------------------------------------
-- 1. CREAR USUARIO ADMIN SI NO EXISTE
-- --------------------------------------------------------
-- Contraseña: admin (SHA256)
INSERT INTO admin_usuario (username, password_hash, rol, activo)
SELECT 'admin', '8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918', 'admin', TRUE
WHERE NOT EXISTS (SELECT 1 FROM admin_usuario WHERE username = 'admin');

-- --------------------------------------------------------
-- 2. INSERTAR TIENDAS DE PRUEBA
-- --------------------------------------------------------
INSERT INTO tiendas (uid, nombre, produccion, desarrollo, activa) VALUES
('TI001', 'Tienda Principal Centro', 'https://api.tienda1.com/produccion', 'https://api.tienda1.com/test', TRUE),
('TI002', 'Tienda Norte', 'https://api.tienda2.com/produccion', 'https://api.tienda2.com/test', TRUE),
('TI003', 'Tienda Sur', 'https://api.tienda3.com/produccion', 'https://api.tienda3.com/test', TRUE),
('TI004', 'Tienda Este', 'https://api.tienda4.com/produccion', 'https://api.tienda4.com/test', TRUE),
('TI005', 'Tienda Oeste', 'https://api.tienda5.com/produccion', 'https://api.tienda5.com/test', TRUE)
ON CONFLICT (uid) DO NOTHING;

-- --------------------------------------------------------
-- 3. CREAR USUARIOS NO-ADMIN
-- --------------------------------------------------------
-- Contraseña: user1 (SHA256)
INSERT INTO admin_usuario (username, password_hash, rol, activo)
SELECT 'operador1', '04f899ac6b83e9a7b3a7d73f77a3e7f2e8f4e7d3c9b8a7f6e5d4c3b2a19080706f5e6d7c8b9a0f1e2d3c4b5a69788796a5b4c3d2e1f09a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2', 'user', TRUE
WHERE NOT EXISTS (SELECT 1 FROM admin_usuario WHERE username = 'operador1');

-- Contraseña: user2 (SHA256)
INSERT INTO admin_usuario (username, password_hash, rol, activo)
SELECT 'operador2', 'ef92b778bafe771e89245b89ecbc08a76a4c5c4a0d1c1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7', 'user', TRUE
WHERE NOT EXISTS (SELECT 1 FROM admin_usuario WHERE username = 'operador2');

-- Contraseña: user3 (SHA256)
INSERT INTO admin_usuario (username, password_hash, rol, activo)
SELECT 'supervisor', 'a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27a3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6', 'user', TRUE
WHERE NOT EXISTS (SELECT 1 FROM admin_usuario WHERE username = 'supervisor');

-- --------------------------------------------------------
-- 4. ASIGNAR TIENDAS A USUARIOS NO-ADMIN
-- --------------------------------------------------------
-- Operador 1: Tiendas Centro y Norte
INSERT INTO admin_usuario_tienda (usuario_id, tienda_uid)
SELECT u.id, 'TI001'
FROM admin_usuario u
WHERE u.username = 'operador1'
AND NOT EXISTS (
    SELECT 1 FROM admin_usuario_tienda ut 
    WHERE ut.usuario_id = u.id AND ut.tienda_uid = 'TI001'
);

INSERT INTO admin_usuario_tienda (usuario_id, tienda_uid)
SELECT u.id, 'TI002'
FROM admin_usuario u
WHERE u.username = 'operador1'
AND NOT EXISTS (
    SELECT 1 FROM admin_usuario_tienda ut 
    WHERE ut.usuario_id = u.id AND ut.tienda_uid = 'TI002'
);

-- Operador 2: Tiendas Sur y Este
INSERT INTO admin_usuario_tienda (usuario_id, tienda_uid)
SELECT u.id, 'TI003'
FROM admin_usuario u
WHERE u.username = 'operador2'
AND NOT EXISTS (
    SELECT 1 FROM admin_usuario_tienda ut 
    WHERE ut.usuario_id = u.id AND ut.tienda_uid = 'TI003'
);

INSERT INTO admin_usuario_tienda (usuario_id, tienda_uid)
SELECT u.id, 'TI004'
FROM admin_usuario u
WHERE u.username = 'operador2'
AND NOT EXISTS (
    SELECT 1 FROM admin_usuario_tienda ut 
    WHERE ut.usuario_id = u.id AND ut.tienda_uid = 'TI004'
);

-- Supervisor: Tienda Oeste
INSERT INTO admin_usuario_tienda (usuario_id, tienda_uid)
SELECT u.id, 'TI005'
FROM admin_usuario u
WHERE u.username = 'supervisor'
AND NOT EXISTS (
    SELECT 1 FROM admin_usuario_tienda ut 
    WHERE ut.usuario_id = u.id AND ut.tienda_uid = 'TI005'
);

-- --------------------------------------------------------
-- 5. VERIFICACIÓN
-- --------------------------------------------------------
SELECT '=== USUARIOS CREADOS ===' as info;
SELECT id, username, rol, activo FROM admin_usuario ORDER BY id;

SELECT '=== TIENDAS CREADAS ===' as info;
SELECT uid, nombre, activa FROM tiendas ORDER BY uid;

SELECT '=== ASIGNACIONES USUARIO-TIENDA ===' as info;
SELECT u.username, u.rol, t.uid, t.nombre
FROM admin_usuario_tienda ut
JOIN admin_usuario u ON ut.usuario_id = u.id
JOIN tiendas t ON ut.tienda_uid = t.uid
ORDER BY u.username, t.uid;

-- =====================================================
-- CREDENCIALES DE PRUEBA
-- =====================================================
-- | Usuario    | Contraseña  | Rol    | Tiendas          |
-- |------------|-------------|--------|------------------|
-- | admin      | admin       | admin  | TODAS            |
-- | operador1  | user1       | user   | TI001, TI002      |
-- | operador2  | user2       | user   | TI003, TI004      |
-- | supervisor | user3 | user | TI005 |
-- =====================================================

-- --------------------------------------------------------
-- 6. INSERTAR MENSAJES DE EJEMPLO
-- --------------------------------------------------------
INSERT INTO msg (texto, tipo, activo) VALUES
('Pago procesado correctamente', 'success', TRUE),
('Pago pendiente de confirmación', 'warning', TRUE),
('Error en el procesamiento del pago', 'error', TRUE),
('Pago duplicado', 'info', TRUE),
('Pago cancelado por el usuario', 'info', TRUE),
('Tiempo de espera agotado', 'warning', TRUE)
ON CONFLICT DO NOTHING;

-- --------------------------------------------------------
INSERTAR URLs DE CALLBACK DE EJEMPLO
-- --------------------------------------------------------
INSERT INTO url (url, activo) VALUES
('https://tienda1.com/callback/pago', TRUE),
('https://tienda2.com/callback/pago', TRUE),
('https://tienda3.com/callback/pago', TRUE)
ON CONFLICT DO NOTHING;