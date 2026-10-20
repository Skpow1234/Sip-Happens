# Tratamiento de datos personales (borrador)

**Estado:** borrador operativo para desarrollo. Publicar en `DATA_PROCESSING_POLICY_URL` antes de tráfico real. Revisar con asesoría legal (Ley 1581 de 2012 y normas complementarias en Colombia).

## Responsable

Sip-Happens (operador del comercio). Contacto de soporte / revocación de consentimiento: dirección configurada en `SUPPORT_CONTACT_EMAIL`.

## Datos que tratamos

- Identificador de cliente derivado de la huella SSH (`SHA256:…`)
- Perfil (nombre, correo, documento) cuando el usuario lo completa
- Direcciones de entrega, carrito, pedidos, suscripciones
- Evidencia de consentimiento de newsletter (IP, User-Agent, tokens de confirmación)
- Registros técnicos (request id, métricas) sin datos de tarjeta — el pago ocurre en Stripe

## Finalidades

- Prestación del servicio de compra por SSH/TUI
- Cumplimiento de obligaciones (mayoría de edad, trazabilidad de pedidos)
- Comunicaciones transaccionales (pago, recibo, envío) y marketing solo con doble opt-in

## Conservación (orientación por defecto)

| Dato | Retención sugerida |
|------|--------------------|
| Pedidos y evidencia de cumplimiento | 7 años |
| Perfil de cliente | 2 años tras el último pedido |
| Logs de solicitud | 90 días |

Ajustar con counsel antes de producción.

## Derechos

Acceso, corrección, eliminación y revocación de consentimiento vía el canal de soporte. Enlaces SIC / privacidad se muestran en el TUI y pies de correo (`PRIVACY_POLICY_URL`, `CONSUMER_RIGHTS_URL`).

## Encargados

Infraestructura de hosting, Stripe (pagos), proveedor SMTP cuando `EMAIL_PROVIDER=smtp`, almacenamiento de objetos (MinIO/S3) para imágenes de producto.
