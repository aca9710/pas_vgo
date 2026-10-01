# -*- coding: utf-8 -*-

import atexit
import json
import random
import string
import threading
import time
import urllib.request
import urllib.error
from collections import deque

from flask import Flask, request, g, jsonify

POOL_TIME = 0.0305

# Colas thread-safe de hecho (append/popleft son atómicos en CPython,
# pero usamos lock explícito para claridad y portabilidad).
pagos = deque()
devoluciones = deque()
datalock = threading.Lock()

# Contador global para OrderId / numero de orden
_numero_lock = threading.Lock()
_numero_counter = [0]

yourthread = None
_stop_flag = threading.Event()


def _next_numero():
    with _numero_lock:
        _numero_counter[0] += 1
        return _numero_counter[0]


def create_app():
    app = Flask(__name__)

    def interrupt():
        _stop_flag.set()
        global yourthread
        if yourthread is not None:
            yourthread.cancel()

    def dostuff():
        global yourthread
        if _stop_flag.is_set():
            return

        with datalock:
            if len(pagos) > 0:
                pago = pagos.popleft()
                print('Notificacion pago')
                # El sleep va DENTRO del lock a proposito en el original,
                # pero eso bloquea el request que quiera encolar. Lo movemos
                # fuera para no bloquear.
                pass
            else:
                pago = None

            if len(devoluciones) > 0:
                devolucion = devoluciones.popleft()
            else:
                devolucion = None

        # Procesamos FUERA del lock para no bloquear a los requests
        if pago is not None:
            time.sleep(1)
            envia_sol_pago(pago)
            print('end Notificacion pago')

        if devolucion is not None:
            envia_devolucion(devolucion)
            print('end Notificacion devolucion')

        if not _stop_flag.is_set():
            yourthread = threading.Timer(POOL_TIME, dostuff, ())
            yourthread.daemon = True
            yourthread.start()

    def envia_devolucion(pago):
        """
        pago esperado (dict):
          {
            'RefundID': str,
            'UrlResponse': str,
            'Source': str,        # opcional
            'ExternalID': str,    # opcional
            ...
          }
        """
        idoperacion = pago['RefundID']
        estado = 3  # siempre OK en el simulador

        # Construimos el dict directamente (evita el JSON roto original)
        enviar = {
            "Status": str(estado),
            "Resultmsg": "0:Operacion Satisfactoria",
            "Success": True,
            "TmId": str(random.randint(190971479, 999709873)),
            "ReferenceRefund": pago.get('ReferenceRefund', 'RM00091115987'),
            "ExternalID": pago.get('ExternalID', 'N4C-9510508965850'),
            "ReferenceRefundTM": pago.get('ReferenceRefundTM', '206470589'),
            "BankId": pago.get('BankId', 'MM03717416987'),
            "RefundID": idoperacion,
        }

        encoded_parms = json.dumps(enviar)
        direccionetecsa = pago['UrlResponse']

        print('Enviando devolucion a %s' % direccionetecsa)
        print('Notificacion : %s' % encoded_parms)

        encab = {'Content-Type': 'application/json'}
        data = encoded_parms.encode('utf-8')
        req = urllib.request.Request(direccionetecsa, data, encab)

        try:
            f = urllib.request.urlopen(req, timeout=60)
            response = f.read().decode('utf-8')
            print('Respuesta devolucion: %s' % repr(response))
            f.close()
        except Exception as e:
            print('Error conectando a %s: %s' % (direccionetecsa, e))
        return

    def envia_sol_pago(pago):
        """
        pago esperado (dict):
          {
            'Phone': str,
            'ExternalId': str,
            'Source': str,
            'UrlResponse': str,
            ...
          }
        """
        estado = 5  # siempre cobrar en el simulador

        enviar = {
            "Status": str(estado),
            "Source": pago['Source'],
            "TmId": str(random.randint(1, 999999999)),
            "Phone": pago['Phone'],
            "ExternalId": pago['ExternalId'],
            "BankId": "MM04293308987",
            "Msg": "Success",
            # El contrato exige Bank como STRING (NotificacionRequest.Bank):
            # mandarlo como int produce 422 en /notificapagos/.
            "Bank": str(random.randint(1, 3)),
        }

        encoded_parms = json.dumps(enviar)
        direccionetecsa = pago['UrlResponse']

        print('Enviando notificacion a %s' % direccionetecsa)
        print('Notificacion : %s' % encoded_parms)

        encab = {'Content-Type': 'application/json'}
        data = encoded_parms.encode('utf-8')
        req = urllib.request.Request(direccionetecsa, data, encab)

        try:
            f = urllib.request.urlopen(req, timeout=30)
            response = f.read().decode('utf-8')
            print('Respuesta : %s' % repr(response))
            time.sleep(2)
            f.close()
        except Exception as e:
            print('Error conectandose a %s: %s' % (direccionetecsa, e))

        return

    def dostuffstart():
        global yourthread
        _stop_flag.clear()
        yourthread = threading.Timer(POOL_TIME, dostuff, ())
        yourthread.daemon = True
        yourthread.start()

    dostuffstart()
    atexit.register(interrupt)
    return app


app = create_app()


# --- Helpers de request (usan g, que es por-request) ---

def get_orden_num():
    """
    Devuelve un numero de orden unico e incremental.
    OJO: el original guardaba el contador en app.config pero lo leia
    desde g, con lo que el incremento no persistia. Ahora usamos
    un contador global con lock.
    """
    return _next_numero()


def id_gennum(size=8, chars=string.digits):
    return ''.join(random.choice(chars) for _ in range(size))


def id_gen(size=8, chars=string.ascii_uppercase[:6] + string.digits):
    return ''.join(random.choice(chars) for _ in range(size))


# --- Rutas ---

@app.route('/')
def hola():
    return '<H1>Simulador de pasarela de pagos</H1>'


@app.route('/payorder/', methods=['POST', 'GET'])
def pay_order():
    respuesta = {"PayOrderResult": {
        "Resultmsg": "Metodo no soportado",
        "Success": False,
        "OrderId": -1
    }}

    if request.method == 'POST':
        pago = request.json['request']
        time.sleep(1)
        respuesta = {"PayOrderResult": {
            "Resultmsg": "Orden insertada satisfactoriamente",
            "Success": True,
            "OrderId": random.randint(1, 10000)
        }}
        pago['result'] = respuesta
        with datalock:
            pagos.append(pago)
        print('end Pay order')
    return jsonify(respuesta)


@app.route('/basura/', methods=['POST', 'GET'])
def basuraok():
    print('Acaba de llegar basura')
    respuesta = {'Status': '1',
                 'Resultmsg': 'default',
                 'Success': True}
    return jsonify(respuesta)


@app.route('/refundPay/', methods=['POST'])
def refundPay():
    print('refund Pay order')
    result = {"RefundPayResult": {
        "Resultmsg": "Metodo no soportado",
        "Success": False,
        "RefundID_Order": "-1"
    }}

    if request.method == 'POST':
        pago = {}
        req = request.json['request']
        print(req)
        for campo in req:
            pago[campo] = req[campo]

        # Generamos un numero (efecto colateral: incrementa el contador)
        get_orden_num()

        result = {"RefundPayResult": {
            "Resultmsg": "esto es un mensaje",
            "Success": True,
            "RefundID_Order": "1234"
        }}
        pago['result'] = result
        with datalock:
            devoluciones.append(pago)
        print('end refund Pay order')
    return jsonify(result)


@app.route('/getStatusOrder/<externalid>/<source>/', methods=['GET'])
def getStatusOrder(externalid, source):
    print('getStatusOrder')
    numero = get_orden_num()
    result = {"GetStatusOrderResult": {
        "Resultmsg": "un mensaje",
        "Success": True,
        "BankId": '1234567',
        "ExternalId": externalid,
        "OrderId": "%s" % numero,
        "Status": 3,
        "TmId": "1212121",
        "Bank": '03'
    }}
    print('end getStatusOrder')
    return jsonify(result)


@app.route('/getStatusRefundOrder/<externalid>/<source>/<tmid>/', methods=['GET'])
def getStatusRefundOrder(externalid, source, tmid):
    print('getStatusRefundOrder')
    get_orden_num()
    result = {"getStatusRefundOrderResult": {
        "Resultmsg": "Orden desrefundida satisfactoriamente",
        "Success": True,
        "BankId": '1234567',
        "ExternalId": externalid,
        "ReferenceRefund": "",
        "TmId": tmid,
        "Status": 3
    }}
    print('end getStatusRefundOrder')
    return jsonify(result)


@app.route('/notiftv/', methods=['POST'])
def notiftv():
    print('notificacion : %s' % request.json['ExternalId'])
    return jsonify({'result': 1})


@app.route('/respuestaasinc/', methods=['POST'])
def respuestaasinc():
    print(request)
    return jsonify({'Status': 3, 'Success': True, 'Resultmsg': 'ok'})


if __name__ == '__main__':
    host = '127.0.0.1'
    port = 15001
    # use_reloader=False para no arrancar el thread dos veces
    app.run(debug=True, host=host, port=port, use_reloader=False)