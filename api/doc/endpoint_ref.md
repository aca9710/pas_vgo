# Referencia de Endpoints

## Índice de Endpoints

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| [`/pago/`](#pago--) | POST | Solicitud de pago síncrono |
| [`/pago_a/`](#pago_a-) | POST | Solicitud de pago asíncrono |
| [`/cancelar/`](#cancelar-) | POST | Cancelar solicitud de pago |
| [`/notificapagos/`](#notificapagos-) | POST | Notificación de estado de pago |
| [`/devolucion/`](#devolucion--) | POST | Solicitud de devolución |
| [`/notificaciondevol/`](#notificaciondevol-) | POST | Notificación de devolución |
| [`/estadoordenpago/{externalid}/{source}/`](#estadoordenpago-externalid-source-) | GET | Consulta estado en ETECSA |
| [`/estadoordenpagolocal/{externalid}/{source}/`](#estadoordenpagolocal-externalid-source-) | GET | Consulta estado local |
| [`/estadodevolucion/{externalid}/{source}/{tmid}`](#estadodevolucion-externalid-source-tmid) | GET | Consulta estado devolución |

---

## Endpoints de Pagos

### `/pago/`

#### URL
```
POST /pago/
```

#### Método
`POST`

#### Entrada

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| Amount | float | Sí | Importe del pago. Debe ser mayor que 0. |
| Phone | string | Sí | Identificador del TPV (Terminal Punto de Venta). Longitud máxima: 10 caracteres. |
| Currency | string | Sí | Código de moneda: CUP, CUC, USD, EUR. |
| Description | string | Sí | Descripción del pago. |
| ExternalId | string | Sí | ID externo con formato: UID-IDOPERACION. Debe contener un guion (-). |
| Source | string | Sí | Origen del pago. |
| ValidTime | string | Sí | Tiempo de validez en segundos (valor entero). |
| UrlResponse | string | Sí | URL de respuesta para notificaciones. |

#### Header

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| username | string | Sí | Nombre de usuario para autenticación. |
| password | string | Sí | Contraseña en formato SHA512 codificado en Base64. |
| source | string | Sí | Código de origen registrado en el sistema. |

#### Descripción

Endpoint síncrono para enviar solicitudes de pago a Transfermóvil (ETECSA). Este endpoint espera la respuesta de ETECSA y permanece en espera hasta recibir la notificación del resultado o hasta que expire el tiempo de validez.

### Flujo de procesamiento

1. Verificar autenticación (headers)
2. Verificar ExternalId duplicado
3. Validar IP del cliente
4. Enviar solicitud a ETECSA (URL_PAGOS)
5. Esperar notificación de estado ( hasta ValidTime )
6. Retornar con Estado y Notificacion

#### Códigos de estado de respuesta

| Código | Significado |
|--------|------------|
| 14 | Transacción duplicada (ExternalId ya existe) |
| 25 | IP no válida |
| 50 | Solicitud no enviada (timeout de conexión) |
| 51 | Solicitud aceptada por Transfermóvil |
| 52 | Solicitud no aceptada por Transfermóvil |
| 53 | Pago notificado (procesado correctamente) |
| 54 | Pago vencido (tiempo de validez expirado) |
| 10 | Error interno |

#### Salida

### Éxito (Estado=53)

| Campo | Tipo | Descripción |
|------|------|-------------|
| ExternalId | string | ID externo proporcionado en la solicitud. |
| Estado | int | Código de estado: 53 (notificado). |
| Msg | string | Mensaje descriptivo del estado. |
| Notificacion | dict | Diccionario con datos de notificación del pago. |

### Otros estados

| Campo | Tipo | Descripción |
|------|------|-------------|
| ExternalId | string | ID externo proporcionado. |
| Estado | int | Código de estado (14, 25, 50, 51, 52, 54, 10). |
| Msg | string | Mensaje descriptivo. |
| Notificacion | dict | Presente solo en Estado=53. |

#### Ejemplo de request

```json
{
    "Amount": 100.00,
    "Phone": "1234567890",
    "Currency": "CUP",
    "Description": "Pago de prueba",
    "ExternalId": "UID123-456",
    "Source": "WEB",
    "ValidTime": "600",
    "UrlResponse": "https://example.com/notificapagos/"
}
```

#### Ejemplo de response (éxito)

```json
{
    "ExternalId": "UID123-456",
    "Estado": 53,
    "Msg": "Pago notificado",
    "Notificacion": {
        "Phone": "1234567890",
        "ExternalId": "UID123-456",
        "TmId": "TM123456",
        "BankId": "BANK001",
        "Status": "3",
        "Source": "WEB",
        "Msg": "Pago exitoso",
        "Bank": "Banco metropolitano"
    }
}
```

#### Errores HTTP

| Código | Descripción |
|--------|-------------|
| 401 Unauthorized | Autenticación inválida (headers faltantes o incorrectos). |
| 422 Unprocessable Entity | Datos de entrada inválidos (validación de schema). |
| 500 Internal Server Error | Error interno del servidor. |

---

### `/pago_a/`

#### URL
```
POST /pago_a/
```

#### Método
`POST`

#### Entrada

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| Amount | float | Sí | Importe del pago. Debe ser mayor que 0. |
| Phone | string | Sí | Identificador del TPV (Terminal Punto de Venta). Longitud máxima: 10 caracteres. |
| Currency | string | Sí | Código de moneda: CUP, CUC, USD, EUR. |
| Description | string | Sí | Descripción del pago. |
| ExternalId | string | Sí | ID externo con formato: UID-IDOPERACION. Debe contener un guion (-). |
| Source | string | Sí | Origen del pago. |
| ValidTime | string | Sí | Tiempo de validez en segundos (valor entero). |
| UrlResponse | string | Sí | URL de respuesta para notificaciones. |

#### Header

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| username | string | Sí | Nombre de usuario para autenticación. |
| password | string | Sí | Contraseña en formato SHA512 codificado en Base64. |
| source | string | Sí | Código de origen registrado en el sistema. |

#### Descripción

Endpoint asincrónico para enviar solicitudes de pago a Transfermóvil (ETECSA). A diferencia del endpoint `/pago/`, este endpoint retorna inmediatamente después de enviar la solicitud a ETECSA sin esperar la notificación del resultado.

El procesamiento de la notificación se realiza en segundo plano mediante el procesador de listas (`procesador_listas`), que verifica el estado de los pagos cada 30 segundos.

### Flujo de procesamiento

1. Verificar autenticación (headers)
2. Verificar ExternalId duplicado en listaid
3. Validar IP del cliente
4. Enviar solicitud a ETECSA (URL_PAGOS)
5. Si ETECSA acepta: agregar a cola de seguimiento con `admpagos.add()` y `admpagos.num_orden()`
6. Retornar inmediatamente con Estado=51 (aceptada) o Estado=52 (rechazada)

#### Códigos de estado de respuesta

| Código | Significado |
|--------|------------|
| 14 | Transacción duplicada (ExternalId ya existe) |
| 25 | IP no válida |
| 50 | Solicitud no enviada (timeout de conexión) |
| 51 | Solicitud aceptada por Transfermóvil |
| 52 | Solicitud no aceptada por Transfermóvil |
| 10 | Error interno |

#### Salida

### Éxito (Estado=51)

| Campo | Tipo | Descripción |
|------|------|-------------|
| ExternalId | string | ID externo proporcionado en la solicitud. |
| Estado | int | Código de estado: 51 (aceptada). |
| Msg | string | Mensaje descriptivo del estado. |
| OrderId | int | ID de orden asignado por ETECSA. |
| ValidTime | string | Tiempo de validez solicitado (eco). |

### Rechazo de ETECSA (Estado=52)

| Campo | Tipo | Descripción |
|------|------|-------------|
| ExternalId | string | ID externo proporcionado en la solicitud. |
| Estado | int | Código de estado: 52 (no aceptada). |
| Msg | string | Mensaje descriptivo del estado. |

### Error de validación

| Campo | Tipo | Descripción |
|------|------|-------------|
| ExternalId | string | ID externo proporcionado (o vacío si falló validación). |
| Estado | int | Código de error (14, 25, 50, 10). |
| Msg | string | Mensaje de error correspondiente. |

#### Ejemplo de request

```json
{
    "Amount": 100.00,
    "Phone": "1234567890",
    "Currency": "CUP",
    "Description": "Pago de prueba",
    "ExternalId": "UID123-456",
    "Source": "WEB",
    "ValidTime": "600",
    "UrlResponse": "https://example.com/notificapagos/"
}
```

#### Ejemplo de response (éxito)

```json
{
    "ExternalId": "UID123-456",
    "Estado": 51,
    "Msg": "Solicitud de pago aceptada por transfermovil",
    "OrderId": 123456,
    "ValidTime": "600"
}
```

#### Errores HTTP

| Código | Descripción |
|--------|------------|
| 401 Unauthorized | Autenticación inválida (headers faltantes o incorrectos). |
| 422 Unprocessable Entity | Datos de entrada inválidos (validación de schema). |
| 500 Internal Server Error | Error interno del servidor. |

#### Notas

- El endpoint no espera la notificación del pago, retorna inmediatamente.
- La notificación se procesa en segundo plano cada 30 segundos.
- Si el ExternalId ya fue usado en `/pago/` o `/pago_a/`, retorna Estado=14.
- El campo `OrderId` solo está presente cuando Estado=51.

---

### `/cancelar/`

#### URL
```
POST /cancelar/
```

#### Método
`POST`

#### Entrada

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| Phone | string | Sí | Identificador del TPV. |
| ExternalId | string | Sí | ID externo con formato: UID-IDOPERACION. Debe contener un guion (-). |

#### Header

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| username | string | Sí | Nombre de usuario para autenticación. |
| password | string | Sí | Contraseña en formato SHA512 codificado en Base64. |
| source | string | Sí | Código de origen registrado en el sistema. |

#### Descripción

Endpoint para cancelar una solicitud de pago que está en proceso. Cancela el pago en la tabla `pago_en_proceso` estableciendo el estado a 11, tmid=-1, bankid=-1, msg=-1.

#### Salida

| Campo | Tipo | Descripción |
|------|------|-------------|
| Estado | int | Código de estado: 11 (cancelado). |
| Error | string | Presente solo si hay error. |

#### Ejemplo de request

```json
{
    "Phone": "1234567890",
    "ExternalId": "UID123-456"
}
```

#### Ejemplo de response

```json
{
    "Estado": 11
}
```

#### Errores HTTP

| Código | Descripción |
|--------|-------------|
| 401 Unauthorized | Autenticación inválida. |
| 422 Unprocessable Entity | Datos de entrada inválidos. |

---

### `/notificapagos/`

#### URL
```
POST /notificapagos/
```

#### Método
`POST`

#### Entrada

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| Phone | string | Sí | Teléfono asociado al pago. |
| ExternalId | string | Sí | ID externo con formato: UID-IDOPERACION. |
| TmId | string | Sí | ID de Transfermóvil. |
| BankId | string | Sí | ID del banco. |
| Status | string | Sí | Estado del pago. |
| Source | string | No | Origen del pago. |
| Msg | string | No | Mensaje del estado. |
| Bank | string | No | Nombre del banco. |

> **Nota de contrato Go:** pydantic declara los 8 campos como requeridos, pero el
> contrato Go (y sus tests) tratan `Source`, `Msg` y `Bank` como opcionales.
> Desviación documentada e intencional; no afecta a clientes existentes.

#### Header

No requiere autenticación (endpoint de callback).

#### Descripción

Endpoint síncrono de notificación de estado del pago. Este endpoint es llamado por ETECSA para notificar el resultado del pago. Actualiza el estado del pago en la base de datos y удаляет el ExternalId de la lista de espera.

#### Códigos de estado de respuesta

| Código | Significado |
|--------|------------|
| 18 | Notificación procesada correctamente |
| 54 | Pago vencido |
| 55 | Pago no existe |
| 10 | Error interno |

#### Salida

| Campo | Tipo | Descripción |
|------|------|-------------|
| Success | bool | Indica si la notificación fue procesada correctamente. |
| Resultmsg | string | Mensaje del resultado. |
| Status | string | Código de estado. |

#### Ejemplo de request

```json
{
    "Phone": "1234567890",
    "ExternalId": "UID123-456",
    "TmId": "TM123456",
    "BankId": "BANK001",
    "Status": "3",
    "Source": "WEB",
    "Msg": "Pago exitoso",
    "Bank": "Banco metropolitano"
}
```

#### Ejemplo de response

```json
{
    "Success": true,
    "Resultmsg": "Pago notificado",
    "Status": "18"
}
```

---

## Endpoints de Devoluciones

### `/devolucion/`

#### URL
```
POST /devolucion/
```

#### Método
`POST`

#### Entrada

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| RefundID | string | Sí | ID de devolución con formato: UID-IDOPERACION. Debe contener un guion (-). |
| Source | string | Sí | Origen de la devolución. |
| Code | string | Sí | Código de la devolución. |
| UrlResponse | string | Sí | URL para notificación de结果. |
| Bank | string | Sí | Banco. |
| importe | float | No | Importe a devolver. Por defecto 0. |
| id_tercero | string | No | ID del tercero. |

#### Header

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| username | string | Sí | Nombre de usuario para autenticación. |
| password | string | Sí | Contraseña en formato SHA512 codificado en Base64. |
| source | string | Sí | Código de origen registrado en el sistema. |

#### Descripción

Endpoint asincrónico para solicitar devolución de un pago. Valida que el importe a devolver no exceda el saldo disponible y envía la solicitud a ETECSA.

### Flujo de procesamiento

1. Verificar autenticación
2. Validar importes (no exceder lo pagado)
3. Validar IP del cliente
4. Insertar en tabla `devolucion_proceso`
5. Enviar solicitud a ETECSA
6. Actualizar estado según respuesta

#### Códigos de estado de respuesta

| Código | Significado |
|--------|------------|
| 17 | Error de respuesta ETECSA |
| 23 | Devolución exitosa |
| 24 | Devolución fallida |
| 25 | IP no válida |
| 37 | Importe excede lo pagado |
| 10 | Error interno |

#### Salida

| Campo | Tipo | Descripción |
|------|------|-------------|
| Estado | int | Código de estado. |
| Msg | string | Mensaje del estado. |
| ETECSA | dict | Respuesta de ETECSA (si disponible). |

#### Ejemplo de request

```json
{
    "RefundID": "UID123-456",
    "Source": "WEB",
    "Code": "01",
    "UrlResponse": "https://example.com/notificaciondevol/",
    "Bank": "Bancometropolitano",
    "importe": 50.00,
    "id_tercero": "UID123-original"
}
```

#### Ejemplo de response

```json
{
    "Estado": 23,
    "Msg": "Devolución aceptada",
    "ETECSA": {
        "Success": true,
        "RefundID_Order": "REF123",
        "Resultmsg": "OK"
    }
}
```

#### Errores HTTP

| Código | Descripción |
|--------|-------------|
| 401 Unauthorized | Autenticación inválida. |
| 422 Unprocessable Entity | Datos de entrada inválidos. |

---

### `/notificaciondevol/`

#### URL
```
POST /notificaciondevol/
```

#### Método
`POST`

#### Entrada

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| RefundID | string | Sí | ID de devolución con formato: UID-IDOPERACION. |
| ReferenceRefund | string | Sí | Referencia de devolución. |
| ReferenceRefundTM | string | Sí | Referencia de Transfermóvil. |
| Success | bool | Sí | Indica si fue exitosa. |
| Resultmsg | string | Sí | Mensaje del resultado. |
| Status | string | Sí | Estado. |
| ExternalID | string | Sí | ID externo del pago original. |
| BankId | string | Sí | ID del banco. |
| TmId | string | Sí | ID de Transfermóvil. |

#### Header

No requiere autenticación (endpoint de callback).

#### Descripción

Endpoint de notificación de devolución. Actualiza el estado en la tabla `devolucion_proceso` y reenvía la notificación a la URL registrada.

#### Códigos de conversión de estado

| Status original | Estado convertido |
|-----------------|-------------------|
| 1 | 30 |
| 2 | 0 |
| 3 | 1 |
| 4 | 31 |
| 5 | 32 |
| 6 | 33 |
| 7 | 34 |
| 8 | 35 |
| 9 | 36 |

#### Salida

| Campo | Tipo | Descripción |
|------|------|-------------|
| Success | bool | Resultado de la operación. |
| Resultmsg | string | Mensaje (presente solo en error). |
| Status | string | Estado (presente solo en error). |

#### Ejemplo de request

```json
{
    "RefundID": "UID123-456",
    "ReferenceRefund": "REF001",
    "ReferenceRefundTM": "TM123456",
    "Success": true,
    "Resultmsg": "OK",
    "Status": "1",
    "ExternalID": "UID123-original",
    "BankId": "BANK001",
    "TmId": "TM123456"
}
```

#### Ejemplo de response

```json
{
    "Success": true
}
```

---

## Endpoints de Estado

### `/estadoordenpago/{externalid}/{source}/`

#### URL
```
GET /estadoordenpago/{externalid}/{source}/
```

#### Método
`GET`

#### Path Parameters

| Campo | Tipo | Descripción |
|-------|------|-------------|
| externalid | string | ID externo con formato: UID-IDOPERACION. |
| source | string | Origen del pago. |

#### Header

No requiere autenticación.

#### Descripción

Endpoint para consultar el estado de una orden de pago en ETECSA. Realiza una petición GET al servicio de ETECSA para obtener el estado actual.

#### Códigos de estado de respuesta

| Código | Significado |
|--------|------------|
| 0 | En proceso |
| 1 | Pendiente |
| 3 | Pagado |
| 5 | Cancelado/Rechazado |
| 7 | Error de conexión |
| 8 | Error de respuesta |

#### Salida

| Campo | Tipo | Descripción |
|------|------|-------------|
| ETECSA | dict | Respuesta completa de ETECSA. |
| MsgAPI | string | Mensaje formateado del API. |
| Estado | int | Código de estado. |
| response | string | Respuesta cruda (si hay error). |

#### Ejemplo de request

```
GET /estadoordenpago/UID123-456/WEB/
```

#### Ejemplo de response

```json
{
    "ETECSA": {
        "Status": 3,
        "OrderId": 123456,
        "Resultmsg": "Pagado"
    },
    "MsgAPI": "3: Pagado",
    "Estado": 3
}
```

---

### `/estadoordenpagolocal/{externalid}/{source}/`

#### URL
```
GET /estadoordenpagolocal/{externalid}/{source}/
```

#### Método
`GET`

#### Path Parameters

| Campo | Tipo | Descripción |
|-------|------|-------------|
| externalid | string | ID externo con formato: UID-IDOPERACION. |
| source | string | Origen del pago. |

#### Header

No requiere autenticación.

#### Descripción

Endpoint para consultar el estado local de una orden de pago. Consulta las tablas `pagos` y `pago_en_proceso` en la base de datos local.

#### Estados locales

| Código | Significado |
|--------|------------|
| 1 | Pendiente |
| 2 | Enviada |
| 3 | Pagada |
| 4 | Aceptada |
| 5 | Procesando |
| 6 | Cancelada |
| 7 | Rechazada |
| 8 | Error |
| 9 | Desconocido |
| -1 | No encontrado |

#### Salida

| Campo | Tipo | Descripción |
|------|------|-------------|
| Msg | string | Mensaje formateado. |
| Local | dict | Datos locales del pago. |
| Estado | int | Código de estado. |

#### Ejemplo de request

```
GET /estadoordenpagolocal/UID123-456/WEB/
```

#### Ejemplo de response

```json
{
    "Msg": "3: Pagada",
    "Local": {
        "OrderId": "123456",
        "Status": 3,
        "Resultmsg": "Pagado",
        "Success": true,
        "TmId": "TM123456",
        "ExternalId": "UID123-456",
        "BankId": "BANK001",
        "Bank": 1,
        "Phone": "1234567890"
    },
    "Estado": 3
}
```

---

### `/estadodevolucion/{externalid}/{source}/{tmid}`

#### URL
```
GET /estadodevolucion/{externalid}/{source}/{tmid}/
```

#### Método
`GET`

#### Path Parameters

| Campo | Tipo | Descripción |
|-------|------|-------------|
| externalid | string | ID externo con formato: UID-IDOPERACION. |
| source | string | Origen del pago. |
| tmid | string | ID de Transfermóvil. |

#### Header

No requiere autenticación.

#### Descripción

Endpoint para consultar el estado de una solicitud de devolución en ETECSA. Realiza una petición GET al servicio de ETECSA.

#### Códigos de estado de respuesta

| Código | Significado |
|--------|------------|
| 26 | Devolución exitosa |
| 27 | Devolución fallida |
| 7 | Error de conexión |

#### Salida

| Campo | Tipo | Descripción |
|------|------|-------------|
| ETECSA | dict | Respuesta de ETECSA. |
| Msg | string | Mensaje formateado. |
| Estado | int | Código de estado. |

#### Ejemplo de request

```
GET /estadodevolucion/UID123-456/WEB/TM123456/
```

#### Ejemplo de response

```json
{
    "ETECSA": {
        "Success": true,
        "RefundID": "REF123",
        "Resultmsg": "OK"
    },
    "Msg": "26: Devolución exitosa",
    "Estado": 26
}
```

---

## Autenticación

Todos los endpoints (excepto `/notificapagos/`, `/notificaciondevol/`, y endpoints de estado) requieren autenticación mediante headers HTTP:

| Header | Descripción |
|--------|-------------|
| username | Nombre de usuario registrado. |
| password | Contraseña codificada en SHA512 y Base64. |
| source | Código de origen registrado en el sistema. |

### Ejemplo de cálculo de password

```python
import hashlib
import base64

password = "mi_contraseña"
password_sha512_base64 = base64.b64encode(hashlib.sha512(password.encode()).digest()).decode()
```

---

## Códigos de Error Comunes

| Código | Descripción |
|--------|------------|
| 10 | Error interno |
| 14 | Transacción duplicada |
| 17 | Error de respuesta ETECSA |
| 18 | Notificación procesada |
| 19 | Error en consulta local |
| 23 | Operación exitosa |
| 24 | Operación fallida |
| 25 | IP no válida |
| 30-36 | Estados de devolución |
| 37 | Importe excedido |
| 50 | Timeout de conexión |
| 51 | Aceptada |
| 52 | No aceptada |
| 53 | Notificada |
| 54 | Vencida |
| 55 | No existe |