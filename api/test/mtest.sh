#!/bin/bash
# test_paralelo.sh — prueba de pago contra la API en ejecucion (puerto 5081).
#
# Uso: ./test/mtest.sh [total] [concurrentes]
#   total:        numero de solicitudes (default 1)
#   concurrentes: solicitudes en paralelo (default 1)
#
# Requiere la API corriendo (go run .) y los headers de autenticacion validos
# (username/password/source) configurados abajo.

URL="http://localhost:5081/pago/"
CONCURRENTES=${2:-1}
TOTAL=${1:-1}

# Credenciales de autenticacion (ajustar al entorno).
AUTH_USERNAME="cimex"
AUTH_PASSWORD="HgamlELo0+RLHbZNpAoTrs96IxZhvqkimrd5tPnhmzlF7kJMbBYNwDmYfxQFf2V1b56rtd5KLJUKYwkTkIi5/Q=="
AUTH_SOURCE="70014"

enviar_pago() {
    local id=$1
    local external_id="BASH${id}-$(uuidgen | cut -d'-' -f1)"

    curl -s -X POST "$URL" \
        -H "Content-Type: application/json" \
        -H "username: $AUTH_USERNAME" \
        -H "password: $AUTH_PASSWORD" \
        -H "source: $AUTH_SOURCE" \
        -d "{
            \"ExternalId\": \"$external_id\",
            \"Currency\": \"USD\",
            \"Phone\": \"5$((RANDOM % 90000000 + 10000000))\",
            \"Amount\": $((RANDOM % 5000 + 100)),
            \"Description\": \"Test $id\",
            \"ValidTime\": 300,
            \"Source\": \"70014\",
            \"UrlResponse\": \"http://localhost:5081/notificapagos/\"
        }"
    echo
}

echo "Iniciando $TOTAL solicitudes con $CONCURRENTES concurrentes..."

for i in $(seq 1 $TOTAL); do
    enviar_pago $i &

    # Limitar concurrencia
    if (( $i % $CONCURRENTES == 0 )); then
        wait
    fi
done

wait
echo "Prueba completada"