package models

import "time"

// Structs para filas de BD (no ORM). Tags `db:` usados por pgx.RowToStructByName.

// Pago corresponde a la tabla `pagos`
type Pago struct {
	ID          int64     `db:"id"`
	IDOperacion string    `db:"idoperacion"`
	UID         *string   `db:"uid"`
	Cliente     string    `db:"cliente"`
	Importe     float64   `db:"importe"`
	Estado      int       `db:"estado"`
	Fecha       time.Time `db:"fecha"`
	OrderID     *int64    `db:"orderid"`
	Msg         *int64    `db:"msg"`
	TMID        *string   `db:"tmid"`
	BankID      *string   `db:"bankid"`
	URL         *int64    `db:"url"`
	// Campos de JOIN (opcionales)
	URLTexto     *string `db:"url_texto"`
	MsgTexto     *string `db:"msg_texto"`
	EstadoNombre string  `db:"estado_nombre"`
}

// Devolucion corresponde a la tabla `devolucion`
type Devolucion struct {
	ID          int64     `db:"id"`
	IDOperacion *string   `db:"idoperacion"`
	UID         *string   `db:"uid"`
	Cliente     string    `db:"cliente"`
	Importe     float64   `db:"importe"`
	Estado      int       `db:"estado"`
	Fecha       time.Time `db:"fecha"`
	ExternalID  *string   `db:"external_id"`
	ResultMsg   *string   `db:"resultmsg"`
}

// Tienda corresponde a la tabla `tiendas`
type Tienda struct {
	UID        string     `db:"uid"`
	Nombre     *string    `db:"nombre"`
	Produccion *string    `db:"produccion"`
	Desarrollo *string    `db:"desarrollo"`
	Activa     bool       `db:"activa"`
	EntidadID  *int64     `db:"entidad_id"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}

// TPV corresponde a la tabla `tpv`
type TPV struct {
	ID          int64     `db:"id"`
	UID         string    `db:"uid"`
	DirIP       string    `db:"dirip"`
	IdentOptima *string   `db:"identoptima"`
	Nombre      *string   `db:"nombre"`
	Activo      bool      `db:"activo"`
	CreatedAt   time.Time `db:"created_at"`
	// JOIN
	TiendaNombre *string `db:"tienda_nombre"`
}

// Entidad corresponde a la tabla `entidad`
type Entidad struct {
	ID           int64     `db:"id"`
	Nombre       string    `db:"nombre"`
	Llave        string    `db:"llave"`
	AdminID      *int64    `db:"admin_id"`
	Activo       bool      `db:"activo"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
	// JOIN / subqueries
	AdminUsername *string `db:"admin_username"`
	NumTiendas    int64   `db:"num_tiendas"`
	NumUsuarios   int64   `db:"num_usuarios"`
}

// AdminUsuario corresponde a la tabla `admin_usuario`
type AdminUsuario struct {
	ID        int64     `db:"id"`
	Username  string    `db:"username"`
	Rol       string    `db:"rol"`
	Activo    bool      `db:"activo"`
	EntidadID *int64    `db:"entidad_id"`
	CreatedAt time.Time `db:"created_at"`
	// Tiendas asignadas (cargadas por separado)
	Tiendas []TiendaRef `db:"-"`
}

// TiendaRef referencia ligera de tienda (para selects y chips)
type TiendaRef struct {
	UID    string  `db:"uid"`
	Nombre *string `db:"nombre"`
}

// NotificacionPago corresponde a la tabla `notificacion_pago`
type NotificacionPago struct {
	ID         int64     `db:"id"`
	Phone      string    `db:"phone"`
	ExternalID string    `db:"external_id"`
	TMID       *string   `db:"tmid"`
	BankID     *string   `db:"bankid"`
	Status     string    `db:"status"`
	Source     *string   `db:"source"`
	Msg        *string   `db:"msg"`
	Bank       *string   `db:"bank"`
	Fecha      time.Time `db:"fecha"`
}

// NotificacionDevolucion corresponde a la tabla `notificacion_devolucion`
type NotificacionDevolucion struct {
	ID                int64     `db:"id"`
	RefundID          string    `db:"refund_id"`
	ReferenceRefund   *string   `db:"reference_refund"`
	ReferenceRefundTM *string   `db:"reference_refund_tm"`
	Success           bool      `db:"success"`
	ResultMsg         *string   `db:"resultmsg"`
	Status            *string   `db:"status"`
	ExternalID        *string   `db:"external_id"`
	BankID            *string   `db:"bankid"`
	TMID              *string   `db:"tmid"`
	Fecha             time.Time `db:"fecha"`
}

// Cliente corresponde a la tabla `cliente`
type Cliente struct {
	ID           int64     `db:"id"`
	Email        string    `db:"email"`
	Nombre       string    `db:"nombre"`
	PasswordHash string    `db:"password_hash"`
	Activo       bool      `db:"activo"`
	CreatedAt    time.Time `db:"created_at"`
}

// Msg corresponde a la tabla `msg`
type Msg struct {
	ID     int64  `db:"id"`
	Texto  string `db:"texto"`
	Tipo   string `db:"tipo"`
	Activo bool   `db:"activo"`
}

// URL corresponde a la tabla `url`
type URL struct {
	ID        int64     `db:"id"`
	URL       string    `db:"url"`
	Activo    bool      `db:"activo"`
	CreatedAt time.Time `db:"created_at"`
}

// SesionAdmin corresponde a la tabla `admin_sesion` (con JOIN a admin_usuario)
type SesionAdmin struct {
	ID        string    `db:"id"`
	UsuarioID int64     `db:"usuario_id"`
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"`
	// JOIN
	Username string `db:"username"`
	Rol      string `db:"rol"`
	Activo   bool   `db:"activo"`
	// Tiendas asignadas (cargadas por separado)
	Tiendas []string `db:"-"`
}

// SesionCliente corresponde a la tabla `sesion_cliente` (con JOIN a cliente)
type SesionCliente struct {
	ID        string    `db:"id"`
	ClienteID int64     `db:"cliente_id"`
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"`
	// JOIN
	ClienteNombre string `db:"cliente_nombre"`
	ClienteEmail  string `db:"cliente_email"`
}

// Mapa de estados de pago (equivalente a ESTADOS de Python)
var Estados = map[int]string{
	0:   "Pendiente",
	11:  "Cancelada",
	14:  "Duplicada",
	15:  "En Proceso",
	18:  "Notificada",
	50:  "No Enviada",
	51:  "Aceptada",
	52:  "No Aceptada",
	53:  "Notificada",
	54:  "Vencida",
	55:  "No Existe",
	500: "Error",
}

// EstadoNombre devuelve el nombre legible de un estado
func EstadoNombre(estado int) string {
	if nombre, ok := Estados[estado]; ok {
		return nombre
	}
	return "Desconocido"
}

// MesesES nombres de meses en español (para reportes)
var MesesES = map[int]string{
	1:  "Enero",
	2:  "Febrero",
	3:  "Marzo",
	4:  "Abril",
	5:  "Mayo",
	6:  "Junio",
	7:  "Julio",
	8:  "Agosto",
	9:  "Septiembre",
	10: "Octubre",
	11: "Noviembre",
	12: "Diciembre",
}