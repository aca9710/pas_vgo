# -*- coding: utf-8 -*-

import base64
import datetime
import hashlib
import json
import random
import ssl
import string
import sys
import time
import traceback
import urllib.request
import urllib.error

#from comun import DataBase


class Cliente:

    def prepara_conexion(self, usuario, password):
        encab = {'Content-Type': 'application/json',
                 'username': usuario,
                 'password': password}
        return encab

    def envia_datos(self, servidor, enviar, usuario, password):
        encoded_parms = json.dumps(enviar)
        encab = self.prepara_conexion(usuario, password)
        req = urllib.request.Request(
            servidor,
            encoded_parms.encode('utf-8'),
            encab
        )
        gcontext = ssl._create_unverified_context()
        response = None
        try:
            f = urllib.request.urlopen(req, timeout=6000, context=gcontext)
            try:
                response = f.read().decode('utf-8')
                f.close()
                response = json.loads(response)
            except Exception:
                exc_type, exc_value, exc_traceback = sys.exc_info()
                txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
                response = {
                    u'Estado': getattr(exc_value, 'errno', -1),
                    'Mensaje': getattr(exc_value, 'strerror', str(exc_value))
                }
        except Exception:
            exc_type, exc_value, exc_traceback = sys.exc_info()
            txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
            response = {u'Estado': -1, 'Mensaje': txt}
        return response

    def enviadatos(self, servidor, datos):
        req = urllib.request.Request(
            url=servidor,
            data=json.dumps(datos).encode('utf-8')
        )
        gcontext = ssl._create_unverified_context()
        response = None
        try:
            f = urllib.request.urlopen(req, timeout=600, context=gcontext)
            try:
                response = f.read().decode('utf-8')
                f.close()
                response = json.loads(response)
            except Exception:
                exc_type, exc_value, exc_traceback = sys.exc_info()
                txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
                response = {
                    u'Estado': getattr(exc_value, 'errno', -1),
                    'Mensaje': getattr(exc_value, 'strerror', str(exc_value))
                }
        except Exception:
            exc_type, exc_value, exc_traceback = sys.exc_info()
            txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
            response = {u'Estado': -1, 'Mensaje': txt}
        return response

    def enviadevol(self, servidor, datos, header):
        encab = header
        req = urllib.request.Request(
            url=servidor,
            data=json.dumps(datos).encode('utf-8'),
            headers=encab
        )
        gcontext = ssl._create_unverified_context()
        response = None
        try:
            f = urllib.request.urlopen(req, timeout=600, context=gcontext)
            try:
                response = f.read().decode('utf-8')
                f.close()
                response = json.loads(response)
            except Exception:
                exc_type, exc_value, exc_traceback = sys.exc_info()
                txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
                response = {
                    u'Estado': getattr(exc_value, 'errno', -1),
                    'Mensaje': getattr(exc_value, 'strerror', str(exc_value))
                }
        except Exception:
            exc_type, exc_value, exc_traceback = sys.exc_info()
            txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
            response = {
                u'Estado': getattr(exc_value, 'errno', -1),
                'Mensaje': getattr(exc_value, 'strerror', str(exc_value))
            }
        return response





def traza(texto=''):
    """
    Guarda error producido por una excepcion.
    """
    _fecha = datetime.date.today()
    txtfecha = _fecha.strftime("%d/%m/%Y ")
    txthora = datetime.datetime.now()
    if txthora.strftime("%H") > '11':
        txthora = txthora.strftime("%I:%M:%S ") + 'p.m.'
    else:
        txthora = txthora.strftime("%I:%M:%S ") + 'a.m.'

    texto = '%s, %s  %s \n' % (txtfecha, txthora, texto)

    try:
        fichero = 'D:\\fuentes\\api_comb\\pruebas\\prueba%s.dat' % _fecha.strftime("_%Y%m%d").replace('\\', '/')
        fichero = fichero.replace('/comun', '').replace('\\comun', '')
        with open(fichero, 'a', encoding='utf-8') as strad:
            strad.write(texto)
    except Exception:
        pass
    return ''


def genera_password(usuario, source, semilla='externalpayment'):
    fecha = datetime.date.today().strftime("%d,%m,%Y").split(',')
    if fecha[0][0] == '0':
        fecha[0] = fecha[0][1]
    if fecha[1][0] == '0':
        fecha[1] = fecha[1][1]
    password = '%s%s%s%s%s%s' % (usuario, fecha[0], fecha[1], fecha[2], semilla, source)
    # hashlib necesita bytes; b64encode devuelve bytes -> decodificamos a ascii
    # para que el header HTTP sea str.
    password = base64.b64encode(
        hashlib.sha512(password.encode('utf-8')).digest()
    ).decode('ascii')
    return password


def prepara_conexion(usuario, source):
    password = genera_password(usuario, source)
    encab = {'Content-Type': 'application/json',
             'username': usuario,
             'source': source,
             'password': password}
    return encab


def envia_sol_pago(url, enviar, usuario='cimex'):
    """
    Envia una solicitud de pago al servidor.
    `enviar` DEBE contener al menos 'Source' y 'ExternalId'.
    """
    servidor = url
    encoded_parms = json.dumps(enviar)
    print('Enviando solicitud de pago a %s' % servidor)

    encab = prepara_conexion(usuario, source=enviar['Source'])

    req = urllib.request.Request(
        servidor,
        encoded_parms.encode('utf-8'),
        encab
    )
    gcontext = ssl._create_unverified_context()

    response = '{}'
    try:
        print('Inicio de la conexion')
        f = urllib.request.urlopen(req, timeout=600, context=gcontext)
        print('Conexion terminada')
        response = f.read().decode('utf-8')
        time.sleep(1)
        f.close()
    except Exception:
        exc_type, exc_value, exc_traceback = sys.exc_info()
        txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
        print(txt)
        return {'Error': 'Ha ocurrido un error al intentar conectarse a %s' % servidor}

    try:
        return json.loads(response)
    except Exception:
        print('Ha ocurrido una excepcion parseando la respuesta de %s' % enviar['ExternalId'])
        exc_type, exc_value, exc_traceback = sys.exc_info()
        txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
        print(txt)
        return {'Error': 'Respuesta no es JSON valido', 'Raw': response}


def id_alfagen(size=87, chars=string.ascii_uppercase + string.digits + string.ascii_lowercase):
    return ''.join(random.choice(chars) for _ in range(size))


def envia_devolucion(url, devolucion, usuario='cimex'):
    """
    Envia una devolucion al servidor.
    `devolucion` DEBE contener al menos 'Source' y 'UrlResponse'.
    No mutamos el dict original: trabajamos sobre una copia.
    """
    enviar = dict(devolucion)  # copia superficial
    # Nos aseguramos de que exista la clave 'request' antes de tocarla
    if 'request' not in enviar or not isinstance(enviar.get('request'), dict):
        enviar['request'] = {}
    enviar['request']['UrlResponse'] = devolucion['UrlResponse']

    encoded_parms = json.dumps(enviar)
    direccionetecsa = url

    encab = prepara_conexion(usuario=usuario, source=devolucion['Source'])
    print('Api intenta conectarse a: %s' % direccionetecsa)
    print('Parametros %s' % encoded_parms)
    print('Encabezado (Header) %s' % encab)

    req = urllib.request.Request(
        direccionetecsa,
        encoded_parms.encode('utf-8'),
        encab
    )
    gcontext = ssl._create_unverified_context()

    response = {}
    estado = 1
    try:
        print('Conectando con %s' % direccionetecsa)
        f = urllib.request.urlopen(req, timeout=10, context=gcontext)
        print('Desconectado')
        response = f.read().decode('utf-8')
        f.close()
        try:
            response = json.loads(response)
        except Exception:
            estado = 28
            print('El JSON recibido no tiene una estructura valida')
            exc_type, exc_value, exc_traceback = sys.exc_info()
            txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
            response = {'Error': {'MSG Error': txt, 'URL': direccionetecsa}}
    except Exception:
        estado = 7
        print('Ha ocurrido un error en la conexion a etecsa')
        exc_type, exc_value, exc_traceback = sys.exc_info()
        txt = repr(traceback.format_exception(exc_type, exc_value, exc_traceback))
        print(txt)
        response = {'Error': {'MSG Error': txt, 'URL': direccionetecsa}}
    return response, estado


if __name__ == '__main__':

    usuario = 'pruebas'
    source = '30079'   # antes se sobreescribia mas abajo; lo dejamos consistente
    serv = "http://127.0.0.1:5081/%s/"
    idoperacion = id_alfagen(10)

    pago = {
        'Amount': 100.00,
        'Phone': '1111111111',
        'Currency': 'USD',
        'Description': '',
        'ExternalId': 'T0001-%s' % idoperacion,
        'Source': source,
        'UrlResponse': '..',
        # El contrato (pydantic del legacy y models.SolicitudPagoRequest) exige
        # ValidTime como STRING: mandarlo como int produce 422
        # "Input should be a valid string". Ademas la API acota el valor a
        # 30..3600 segundos.
        'ValidTime': '600'
    }

    # Ejemplo de solicitud de pago
    url = serv % 'pago'
    respuesta = envia_sol_pago(url, pago, usuario=usuario)
    print(json.dumps(respuesta, indent=2, ensure_ascii=False))

    # Ejemplo de devolucion (descomentar para probar)
    # devolucion = {
    #     'RefundID': pago['ExternalId'],
    #     'Source': pago['Source'],
    #     'Code': '',
    #     'UrlResponse': pago['UrlResponse'],
    #     'Bank': '1',
    #     'request': {}
    # }
    # respuestadev, estado = envia_devolucion(serv % 'devolucion', devolucion, usuario=usuario)
    # print(json.dumps(respuestadev, indent=2, ensure_ascii=False), 'estado=', estado)