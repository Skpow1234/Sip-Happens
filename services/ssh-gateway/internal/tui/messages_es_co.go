package tui

import "fmt"

// SpanishCO returns Colombia-first Spanish copy for all TUI screens.
func SpanishCO() Messages {
	return Messages{
		AppName:             "Sip-Happens",
		Loading:             "Cargando…",
		LoadingProduct:      "Cargando producto…",
		MinorAlcoholWarning: "Prohibida la venta de alcohol a menores de edad.",
		WelcomeBirthPrompt:  "Ingresa tu fecha de nacimiento (AAAA-MM-DD):",
		WelcomeContinue:     "enter — continuar   n — salir",
		WelcomePrivacyLabel:        "Política de privacidad:",
		WelcomeConsumerRightsLabel: "Derechos del consumidor (SIC):",
		MenuTitle:                  "Menú principal",
		MenuCart: func(n int) string {
			return fmt.Sprintf("Carrito: %d artículo(s)", n)
		},
		MenuLastOrder: "Último pedido:",
		MenuHelp: `
  1 — Ver catálogo
  2 — Ver carrito
  3 — Estado del último pedido
  4 — Historial de pedidos
  ? — Ayuda
  q — Salir

  Privacidad: %s
  Derechos del consumidor (SIC): %s
`,
		CatalogTitle: "Catálogo",
		CatalogPageMeta: func(page, offset int) string {
			return fmt.Sprintf("página %d  desplazamiento %d", page, offset)
		},
		CatalogSearchPrompt: "Buscar:",
		CatalogSearchApply:  "enter aplicar  esc cancelar",
		CatalogNoMatches:    "(Sin resultados — ajusta filtros o pulsa r)",
		CatalogHelp:         "↑/↓  enter  ←/→ cantidad  b carrito  / buscar  f filtros  n/p página  r actualizar  esc menú",
		CatalogAvail:        "disp",
		ProductSKU:          "SKU",
		ProductABV:          "%.1f%% ABV",
		ProductVolume:       "%d ml",
		ProductInvima:       "INVIMA",
		ProductDescription:  "Descripción",
		ProductIBU:          "IBU",
		ProductPrice:        "Precio",
		ProductAvailable:    "Disponible",
		ProductHelp:         "→ agregar   ← quitar   b — carrito   esc — volver",
		CartTitle:           "Carrito",
		CartEmpty:           "(Vacío)",
		CartMaxQty: func(max int) string {
			return fmt.Sprintf("  máx %d", max)
		},
		CartSubtotal: "Subtotal:",
		CartShippingEst: func(city, err string) string {
			return fmt.Sprintf("Envío (est.): —  (%s)", err)
		},
		CartShippingEstValue: func(city, amount string) string {
			return fmt.Sprintf("Envío (est., %s): %s", city, amount)
		},
		CartDeliveryCities: "Ciudades de entrega: Bogotá, Medellín, Cali — se estima en el pago.",
		CartHelp:           "+/- cantidad  d quitar  c pagar  esc menú",
		CheckoutTitle:      "Datos de entrega",
		CheckoutFieldLabels: []string{
			"Ciudad", "Barrio", "Dirección", "Teléfono", "Documento", "Nombre completo",
		},
		CheckoutHelp:       "tab / enter — siguiente campo  esc — volver al carrito",
		CheckoutCitiesHint: "Ciudades de entrega: Bogotá, Medellín o Cali.",
		CheckoutShippingErr: func(err string) string {
			return fmt.Sprintf("Estimación de envío: —  (%s)", err)
		},
		CheckoutShippingWait: "Estimación de envío: (ingresa ciudad)",
		CheckoutShippingEst: func(amount string) string {
			return fmt.Sprintf("Estimación de envío: %s", amount)
		},
		CheckoutConsumerRightsLine: func(url string) string {
			return fmt.Sprintf("Derechos del consumidor (SIC): %s", url)
		},
		ConfirmTitle: "Confirmar pedido",
		ConfirmSubtotal: "Subtotal:",
		ConfirmShipping: "Envío (est.):",
		ConfirmTotal:    "Total:",
		ConfirmItems:    "Artículos:",
		ConfirmPrivacyLine: func(checked bool, url string) string {
			mark := "[ ]"
			if checked {
				mark = "[x]"
			}
			return fmt.Sprintf("%s Acepto la política de privacidad (%s)", mark, url)
		},
		ConfirmConsumerRightsLine: func(url string) string {
			return fmt.Sprintf("Información sobre derechos del consumidor (SIC): %s", url)
		},
		ConfirmDataProcessingLine: func(url string) string {
			return fmt.Sprintf("Tratamiento de datos personales (Ley 1581): %s", url)
		},
		ConfirmHelp: "p — política de privacidad   y — confirmar pedido   n — volver",
		PaymentTitle:  "Pago fuera de la terminal",
		PaymentOrder:  "Pedido:",
		PaymentStatus: "Estado:",
		PaymentPayWithin: func(cd string) string {
			return fmt.Sprintf("Paga en: %s\n\n", cd)
		},
		PaymentReservationTTL: func(until string) string {
			return fmt.Sprintf("Reserva de inventario hasta: %s (si no pagas a tiempo, el pedido se cancela y el stock se libera).\n\n", until)
		},
		PaymentQRHint: "Escanea el código QR o abre esta URL en tu navegador (no ingreses tarjetas por SSH):",
		PaymentCopyTip: "Consejo: selecciona las líneas de la URL (ratón o copiar del terminal) y pégala en el navegador.",
		PaymentHelp:    "o — actualizar estado   esc — menú",
		OrderCompleteTitle: "Pago recibido",
		OrderCompleteBody: func(order, total, status, eta, support string) string {
			return fmt.Sprintf(`¡Gracias! Tu pedido está confirmado.

Pedido: %s
Total: %s
Estado: %s

Despacho: %s

Soporte: %s

Revisa el estado desde el menú principal (3 o 4).

enter / esc — volver al menú
`, order, total, status, eta, support)
		},
		OrderCompleteConsumerRightsLine: func(url string) string {
			return fmt.Sprintf("Derechos del consumidor (SIC): %s", url)
		},
		HelpTitle: "Ayuda",
		HelpBack:  "esc — volver al menú",
		HelpBody: `Sip-Happens — atajos de teclado

Global
  esc     volver / menú
  q       salir (bienvenida, menú, error)
  ?       esta ayuda (desde el menú)

Bienvenida
  enter / n   verificación de edad

Menú principal
  1       catálogo
  2       carrito
  3       último pedido
  4       historial
  ?       ayuda

Catálogo
  ↑/↓     mover
  enter   detalle
  ← / →   quitar / agregar
  b       carrito
  /       buscar
  f       filtros
  n / p   página
  r       actualizar

Carrito
  +/-     cantidad
  d       quitar
  c       pagar

Pago
  o       estado del pedido
  esc     menú

Pedidos
  r       actualizar
`,
		FiltersTitle: "Filtros del catálogo",
		FilterFieldLabels: []string{
			"Estilo", "Cervecería", "ABV mín %", "ABV máx %",
		},
		FiltersHelp: "tab / enter — siguiente  esc — volver sin guardar  y — aplicar",
		HistoryTitle: "Historial de pedidos",
		HistoryEmpty: "(Aún no hay pedidos)",
		HistoryHelp:  "↑/↓ mover  enter ver  r actualizar  esc menú",
		StatusTitle:  "Estado del pedido",
		StatusOrder:  "Pedido:",
		StatusPaymentExpires: func(cd string) string {
			return fmt.Sprintf("El pago vence en: %s\n\n", cd)
		},
		StatusHelp:        "r actualizar  esc menú",
		StatusCancelled:   "El pedido fue cancelado o venció.",
		ErrorTitle:        "Error",
		ErrorBack:         "esc — menú   q — salir",
		StatusBarLoading:  "cargando…",
		StatusBarCart: func(n int) string {
			return fmt.Sprintf("carrito %d", n)
		},
		StatusBarOrder: func(label string) string {
			return "pedido " + label
		},
		ShipmentLabel:     "Envío:",
		ShipmentCarrier:   "Transportadora:",
		ShipmentTracking:  "Guía:",
		ShipmentTrackHint: "Rastrea tu paquete con el código anterior.",
		ShipmentPaid:      "Comenzaremos el despacho en breve.",
		ShipmentFulfilling: "Tu pedido se está preparando para envío.",
		ShipmentShipped:   "Tu pedido fue enviado. La guía aparecerá cuando el transportador escanee.",
		ShipmentPacked:     "Pedido empacado y listo para entregar al transportador.",
		ShipmentHandedOver: "Entregado al transportador; en ruta a tu dirección.",
		ShipmentFailed:     "El envío falló. Soporte se pondrá en contacto contigo.",
		ShipmentStatusLabel: func(status string) string {
			switch status {
			case "PENDING":
				return "pendiente"
			case "PACKED":
				return "empacado"
			case "HANDED_OVER":
				return "entregado al transportador"
			case "IN_TRANSIT":
				return "en ruta"
			case "DELIVERED":
				return "entregado"
			case "FAILED":
				return "fallido"
			default:
				return status
			}
		},
		Expired:           "vencido",
		ErrBirthRequired:  "ingresa tu fecha de nacimiento (AAAA-MM-DD)",
		ErrBirthFormat:    "usa el formato AAAA-MM-DD (ej. 1995-06-15)",
		ErrBirthMinor:     "debes ser mayor de 18 años para usar Sip-Happens",
		ErrCartEmpty:      "el carrito está vacío",
		ErrCartInvalidQty: func(sku string) string {
			return fmt.Sprintf("%s: cantidad inválida", sku)
		},
		ErrCartStock: func(sku string, avail, cart int) string {
			return fmt.Sprintf("%s: solo hay %d disponibles (tienes %d en el carrito)", sku, avail, cart)
		},
		ErrOutOfStock:      "sin inventario",
		ErrPrivacyRequired: "acepta la política de privacidad (pulsa p) antes de continuar",
		ErrCartLineStock: func(sku string, max int) string {
			return fmt.Sprintf("%s: solo %d disponibles", sku, max)
		},
		ErrCheckoutFields:    "completa todos los campos de entrega (ciudad: Bogotá, Medellín o Cali)",
		ErrNoRecentOrder:     "no hay pedido reciente en esta sesión",
		ErrOrderNotInSession: "el pedido no está disponible en esta sesión",
		ErrInvalidABV: func(s string) string {
			return fmt.Sprintf("ABV inválido %q", s)
		},

		ShellBrand:   "Sip-Happens",
		ShellShop:    "tienda",
		ShellAccount: "cuenta",
		ShellCart:    "carrito",
		ShellPromo:   "Envío gratis en pedidos mayores a $160.000 COP (ciudades piloto).",
		CatalogFeatured:   "destacados",
		CatalogAll:        "catálogo",
		CatalogSelectHint: "Selecciona una cerveza de la lista.",
		CatalogFooter:     "↑/↓ productos  ←/→ agregar  b suscribirse  / buscar  f filtros  n/p página  c carrito  ? ayuda",
		ProductDetailFooter: "← quitar  → agregar  b suscribirse",
		CrumbCart:     "carrito",
		CrumbShipping: "envío",
		CrumbPayment:  "pago",
		CrumbConfirm:  "confirmación",
		CartFooter:    "esc atrás  ↑/↓ artículos  +/- cantidad  x quitar  enter pagar",
		CheckoutFooter: "esc carrito  ↑/↓ dirección  enter/tab campos  enter confirmar",
		ConfirmFooter:  "esc envío  p privacidad  y confirmar pedido",
		PaymentFooter:  "o estado  esc tienda",
		PaymentOffTerminal: "No ingreses tarjetas por SSH. Paga en el navegador o escanea el QR.",
		AccountOrders:          "historial de pedidos",
		AccountSubscriptions:   "mis suscripciones",
		AccountSubscriptionsEmpty: "(Sin suscripciones — suscríbete desde la tienda con b)",
		AccountSubscriptionsHelp:  "enter ver  x cancelar  r actualizar",
		SubscriptionConfirmTitle:    "Suscripción recurrente",
		SubscriptionConfirmProduct: func(name, price, interval string, qty int) string {
			return fmt.Sprintf("%s\n%s × %d  ·  cada %s", name, price, qty, interval)
		},
		SubscriptionConfirmAddress: func(label string) string {
			return "Entrega: " + label
		},
		SubscriptionConfirmNoAddress: "(Sin dirección guardada — se puede agregar después del primer pago)",
		SubscriptionConfirmFooter:    "w semanal  m mensual  +/- cantidad  ↑/↓ dirección  y confirmar  esc tienda",
		SubscriptionStatusTitle:      "Suscripción",
		SubscriptionStatusHelp:       "x cancelar  r actualizar  esc cuenta",
		SubscriptionStatusCancel:     "Suscripción cancelada.",
		SubscriptionPaymentOrder:       "Suscripción",
		SubscriptionIntervalWeek:       "semana",
		SubscriptionIntervalMonth:      "mes",
		AccountAddresses:       "direcciones guardadas",
		AccountAddressesEmpty:  "(Sin direcciones guardadas — completa el pago para guardar una)",
		AccountAddressesHelp:   "r actualizar",
		AccountTokens:          "tokens de API",
		AccountTokensEmpty:     "(Sin tokens — pulsa n para crear uno para tus scripts)",
		AccountTokensHelp:      "n nuevo  x revocar  r actualizar",
		AccountTokensListHeader: func(prefix, name string, scopes int, status string) string {
			return fmt.Sprintf("pat_%s…  %s  ·  %d permisos  ·  %s", prefix, name, scopes, status)
		},
		AccountTokenStatusActive:  "activo",
		AccountTokenStatusRevoked: "revocado",
		AccountTokenStatusExpired: "vencido",
		TokenCreateTitle:          "Crear token de API personal",
		TokenCreateNameLabel:      "Nombre (script, equipo, etc.):",
		TokenCreateScopesLabel:    "Permisos:",
		TokenCreatePresetRead:     "solo lectura (catálogo, perfil, pedidos, suscripciones)",
		TokenCreatePresetFull:     "acceso completo (lectura + carrito + pedidos + suscripciones + perfil)",
		TokenCreateFooter:         "1 solo lectura  2 acceso completo  enter editar nombre  y crear  esc cancelar",
		TokenShowTitle:            "Token creado",
		TokenShowOnceWarning:      "⚠ Cópialo ahora. El servidor solo guarda un hash SHA-256 — no podrás verlo otra vez.",
		TokenShowPrefixLabel:      "Prefijo (visible en la lista):",
		TokenShowScopesLabel:      "Permisos:",
		TokenShowSecretLabel:      "Token (úsalo como Bearer):",
		TokenShowCopyHint:         "Consejo: selecciona la línea del token (ratón / Shift+selección) y pégala en tu gestor de secretos.",
		TokenShowFooter:           "esc volver a tokens",
		TokenRevokedNotice:        "Token revocado.",
		ErrTokenNameRequired:      "ingresa un nombre para el token (1-80 caracteres)",
		CheckoutSavedAddresses: "Dirección guardada:",
		CheckoutAddressPickHelp: "↑/↓ elegir  enter confirmar  e editar manual  tab campos",
		CheckoutAddressRePick:   "e elegir dirección guardada",
		AccountFAQ:    "preguntas frecuentes",
		AccountAbout:  "acerca de",
		AccountFAQBody: `Preguntas frecuentes (piloto)

• ¿Cómo pago? Enlace seguro Stripe fuera de la terminal.
• ¿Entregas? Bogotá, Medellín y Cali (v1).
• ¿Edad? Debes ser mayor de 18 años.
• Soporte: usa el contacto en la pantalla de confirmación.`,
		AccountAboutBody: `Sip-Happens — cerveza artesanal en Colombia.

Política de privacidad: %s
Derechos del consumidor (SIC): %s`,
		AccountFooter:     "↑/↓ menú  enter seleccionar  esc tienda",
		HistoryPaneHelp:   "enter ver pedido  r actualizar",

		SplashTagline:          "cerveza artesanal · Colombia",
		ProfileLinkBanner:      "Tu llave SSH aún no está vinculada a un perfil completo — entra a 'cuenta' y completa tus datos para acelerar el pago.",
		PaymentExpiredBanner:   "La sesión de pago de Stripe venció y el inventario fue liberado.",
		PaymentExpiredHelp:     "c — empezar de nuevo desde el carrito   esc — tienda",
		PaymentRefreshing:      "actualizando…",
		CatalogGroupDestacados: "destacados",
		CatalogGroupAll:        "catálogo",
		CatalogGroupToggleHint: "g destacados/catálogo",
		ProductImageLabel:      "Imagen:",
	}
}
