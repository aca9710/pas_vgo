# Manual de Usuario - Pasarela de Pagos

## 1. Introducción

La Pasarela de Pagos es un sistema de administración para gestionar pagos electrónicos, tiendas, TPVs y estadísticas de transacciones.

### 1.1 Requisitos
- Navegador web moderno (Chrome, Firefox, Edge, Safari)
- Conexión al servidor donde está desplegada la aplicación

### 1.2 Acceso
```
URL Admin: http://[servidor]:8000/admin
URL API: http://[servidor]:8000/docs
Portal Cliente: http://[servidor]:8000/login
```

---

## 2. Dashboard Principal

Al acceder a `/admin` verás el dashboard con:
- **Total pagos**: Cantidad de pagos últimos 30 días
- **Pagos exitosos**: Pagos aceptados (estado 51, 53)
- **Monto total**: Suma de importes exitoso
- **Pagos en proceso**: Estados pendientes (15)
- **Últimos pagos**: Lista de 20 operaciones recientes
- **Distribución por estado**: Gráfico de estados

---

## 3. Gestión de Pagos

### 3.1 Listar Pagos
**Ruta**: `/admin/pagos`

**Filtros disponibles**:
| Campo | Descripción |
|-------|-------------|
| idoperacion | ID de operación específico |
| cliente | Email del cliente |
| estado | Código de estado (0-500) |
| fecha_desde | Fecha inicial |
| fecha_hasta | Fecha final |

**Paginación**: Usa los botones "Anterior" / "Siguiente" o cambia el parámetro `page`

### 3.2 Editar Pago
**Ruta**: `/admin/pagos/{id}/editar`

Campos editables:
- **Estado**: Código numérico (51=Aceptada, 52=No Aceptada, etc.)
- **OrderID**: ID de orden en ETECSA
- **Msg**: Mensaje del sistema
- **TmID**: ID de Transfermóvil
- **BankID**: ID del banco

### 3.3 Pagos en Proceso
**Ruta**: `/admin/pagos/proceso`

Muestra pagos con estado 50 (No Enviada) que requieren atención manual.

### 3.4 Detalle de Pago (API)
**Ruta**: `/admin/pagos/{id}`

Retorna los datos completos del pago en formato JSON.

---

## 4. Devoluciones

### 4.1 Listar Devoluciones
**Ruta**: `/admin/devoluciones`

Muestra pagos con estados 51, 52, 53 (aceptada/no aceptada/notificada).

### 4.2 Devoluciones en Proceso
**Ruta**: `/admin/devoluciones/proceso`

Muestra devoluciones pendientes (estado 50).

---

## 5. Notificaciones

### 5.1 Notificaciones de Pago
**Ruta**: `/admin/notificaciones`

Muestra notificaciones recibidas de los bancos:
- Teléfono (Phone)
- ID Externo (ExternalId)
- ID Transfermóvil (TmId)
- ID Banco (BankId)
- Estado (Status)
- Banco (Bank)
- Fecha

### 5.2 Notificaciones de Devolución
**Ruta**: `/admin/notificaciones/devolucion`

Muestra notificaciones de devolución:
- ID Devolución (RefundID)
- Referencia
- Éxito (Success)
- Mensaje (Resultmsg)
- Estado

---

## 6. Tiendas

### 6.1 Listar Tiendas
**Ruta**: `/admin/tiendas`

Tabla con todas las tiendas registradas:
- UID (identificador único)
- Nombre
- URL Producción
- URL Desarrollo
- Activa (sí/no)

### 6.2 Crear Tienda
1. Ir a `/admin/tiendas`
2. Completar formulario:
   - **UID**: Identificador único (máx 20 caracteres)
   - **Nombre**: Nombre de la tienda
   - **Producción**: URL de producción
   - **Desarrollo**: URL de desarrollo
   - **Activa**: Marcar si está activa

### 6.3 Editar Tienda
**Ruta**: `/admin/tiendas/{uid}/editar`

Modifica los datos de una tienda existente.

### 6.4 Eliminar Tienda
Desde la lista de tiendas, usar el botón "Eliminar".

---

## 7. TPV (Terminal de Punto de Venta)

### 7.1 Listar TPVs
**Ruta**: `/admin/tpv`

Tabla con:
- ID
- UID Tienda
- Dirección IP
- ID Optima (opcional)
- Nombre
- Activo

### 7.2 Crear TPV
1. Ir a `/admin/tpv`
2. Formulario:
   - **UID**: ID de la tienda
   - **Dirección IP**: IP del terminal
   - **ID Optima**: ID de caja fiscal (opcional)
   - **Nombre**: Identificador descriptivo
   - **Activo**: Marcar si está activo

### 7.3 Editar/Eliminar TPV
Usar botones en la lista de TPVs.

---

## 8. Estadísticas

### 8.1 Dashboard
**Ruta**: `/admin/estadisticas`

**Tipos de vista**:
| Tipo | Descripción |
|------|-------------|
| mensual | Por mes del año actual |
| diario | Últimos 30 días |
| tiendas | Top 10 tiendas |

**Parámetros**:
- `anio`: Año para vista mensual (default 2026)
- `tipo`: Tipo de gráfica (mensual/diario/tiendas)

**Gráficas**:
- Cantidad de pagos
- Monto total
- Top 5 tiendas
- Distribución por estado

---

## 9. Portal Cliente

### 9.1 Acceso
**Ruta**: `/login`

### 9.2 Login
1. Ingresar email y contraseña
2. Click "Iniciar sesión"
3. Se crea cookie de sesión (válida 7 días)

### 9.3 Portal
**Ruta**: `/portal`

Muestra:
- Nombre del cliente
- Pagos recientes (últimos 20)
- Devoluciones recientes (últimos 20)

### 9.4 Mis Pagos
**Ruta**: `/portal/pagos`

Lista de pagos del cliente autenticado.

### 9.5 Detalle Pago
**Ruta**: `/portal/pagos/{id}`

Información completa de un pago específico.

### 9.6 Mis Devoluciones
**Ruta**: `/portal/devoluciones`

Lista de devoluciones del cliente.

### 9.7 Detalle Devolución
**Ruta**: `/portal/devoluciones/{id}`

Detalles de una devolución específica.

### 9.8 Cerrar Sesión
**Ruta**: `/logout`

Cierra la sesión y redirige a login.

---

## 10. Códigos de Estado

| Código | Significado | Acción |
|--------|------------|--------|
| 0 | Pendiente | Esperando procesamiento |
| 11 | Cancelada | Cancelada por usuario |
| 14 | Duplicada | Duplicada |
| 15 | En Proceso | Procesando |
| 18 | Notificada | Notificación enviada |
| 50 | No Enviada | Requiere reintento |
| 51 | Aceptada | ✅ Éxito |
| 52 | No Aceptada | ❌ Fallida |
| 53 | Notificada | ✅ Confirmada |
| 54 | Vencida | ⏰ Expirada |
| 55 | No Existe | Orden inválida |
| 500 | Error | ⚠️ Error del sistema |

---

## 11. Glosario

| Término | Definición |
|--------|------------|
| TPV | Terminal de Punto de Venta |
| UID | Identificador único de tienda |
| ExternalId | ID externo (formato: UID-IDOPERACION) |
| ETECSA | Entidad procesadora de pagos |
| Transfermóvil | Sistema de pagos móviles |
| callback | Notificación de resultado |
| asíncrono | Pago que notifica después |
| Síncrono | Pago que espera respuesta inmediata |
| PCPOS | Payment Callback POS - pago síncrono |

---

*Manual de usuario - Pasarela de Pagos v1.0*