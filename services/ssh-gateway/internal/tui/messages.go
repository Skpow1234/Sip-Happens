package tui

// Messages holds user-visible TUI copy (Colombia-first Spanish by default).
type Messages struct {
	AppName              string
	Loading              string
	LoadingProduct       string
	MinorAlcoholWarning  string
	WelcomeBirthPrompt   string
	WelcomeContinue      string
	WelcomeExit          string
	WelcomePrivacyLabel       string
	WelcomeConsumerRightsLabel string
	MenuTitle                 string
	MenuCart             func(n int) string
	MenuLastOrder        string
	MenuHelp             string
	CatalogTitle         string
	CatalogPageMeta      func(page, offset int) string
	CatalogSearchPrompt  string
	CatalogSearchApply   string
	CatalogNoMatches     string
	CatalogHelp          string
	CatalogAvail         string
	ProductSKU           string
	ProductABV           string
	ProductVolume        string
	ProductInvima        string
	ProductDescription   string
	ProductIBU           string
	ProductPrice         string
	ProductAvailable     string
	ProductHelp          string
	CartTitle            string
	CartEmpty            string
	CartMaxQty           func(max int) string
	CartSubtotal         string
	CartShippingEst      func(city, err string) string
	CartShippingEstValue func(city, amount string) string
	CartDeliveryCities   string
	CartHelp             string
	CheckoutTitle        string
	CheckoutFieldLabels  []string
	CheckoutHelp         string
	CheckoutCitiesHint   string
	CheckoutShippingErr  func(err string) string
	CheckoutShippingWait string
	CheckoutShippingEst       func(amount string) string
	CheckoutConsumerRightsLine func(url string) string
	ConfirmTitle              string
	ConfirmSubtotal      string
	ConfirmShipping      string
	ConfirmTotal         string
	ConfirmItems         string
	ConfirmPrivacyLine         func(checked bool, url string) string
	ConfirmConsumerRightsLine    func(url string) string
	ConfirmDataProcessingLine    func(url string) string
	ConfirmHelp                  string
	PaymentTitle         string
	PaymentOrder         string
	PaymentStatus        string
	PaymentPayWithin        func(cd string) string
	PaymentReservationTTL   func(until string) string
	PaymentQRHint           string
	PaymentCopyTip       string
	PaymentHelp          string
	OrderCompleteTitle   string
	OrderCompleteBody              func(order, total, status, eta, support string) string
	OrderCompleteConsumerRightsLine func(url string) string
	HelpTitle            string
	HelpBack             string
	HelpBody             string
	FiltersTitle         string
	FilterFieldLabels    []string
	FiltersHelp          string
	HistoryTitle         string
	HistoryEmpty         string
	HistoryHelp          string
	StatusTitle          string
	StatusOrder          string
	StatusPaymentExpires func(cd string) string
	StatusHelp           string
	StatusCancelled      string
	ErrorTitle           string
	ErrorBack            string
	StatusBarLoading     string
	StatusBarCart        func(n int) string
	StatusBarOrder       func(label string) string
	ShipmentLabel        string
	ShipmentCarrier      string
	ShipmentTracking     string
	ShipmentTrackHint    string
	ShipmentPaid         string
	ShipmentFulfilling   string
	ShipmentShipped      string
	ShipmentPacked       string
	ShipmentHandedOver   string
	ShipmentFailed       string
	// ShipmentStatusLabel translates raw shipment status codes
	// (PENDING / PACKED / HANDED_OVER / IN_TRANSIT / DELIVERED / FAILED)
	// into user-visible Spanish labels. Unknown codes pass through as-is.
	ShipmentStatusLabel  func(status string) string
	Expired              string
	ErrBirthRequired     string
	ErrBirthFormat       string
	ErrBirthMinor        string
	ErrCartEmpty         string
	ErrCartInvalidQty    func(sku string) string
	ErrCartStock         func(sku string, avail, cart int) string
	ErrOutOfStock        string
	ErrPrivacyRequired   string
	ErrCartLineStock     func(sku string, max int) string
	ErrCheckoutFields    string
	ErrNoRecentOrder     string
	ErrOrderNotInSession string
	ErrInvalidABV        func(s string) string

	// Shell (TUI v2)
	ShellBrand            string
	ShellShop             string
	ShellAccount          string
	ShellCart             string
	ShellPromo            string
	CatalogFeatured       string
	CatalogAll            string
	CatalogSelectHint     string
	CatalogFooter         string
	ProductDetailFooter   string
	CrumbCart             string
	CrumbShipping         string
	CrumbPayment          string
	CrumbConfirm          string
	CartFooter            string
	CheckoutFooter        string
	ConfirmFooter         string
	PaymentFooter         string
	PaymentOffTerminal    string
	AccountOrders         string
	AccountSubscriptions  string
	AccountSubscriptionsEmpty string
	AccountSubscriptionsHelp  string
	SubscriptionConfirmTitle  string
	SubscriptionConfirmProduct func(name, price, interval string, qty int) string
	SubscriptionConfirmAddress func(label string) string
	SubscriptionConfirmNoAddress string
	SubscriptionConfirmFooter    string
	SubscriptionStatusTitle      string
	SubscriptionStatusHelp       string
	SubscriptionStatusCancel     string
	SubscriptionPaymentOrder     string
	SubscriptionIntervalWeek     string
	SubscriptionIntervalMonth    string
	AccountAddresses      string
	AccountAddressesEmpty string
	AccountAddressesHelp  string
	AccountTokens             string
	AccountTokensEmpty        string
	AccountTokensHelp         string
	AccountTokensListHeader   func(prefix, name string, scopes int, status string) string
	AccountTokenStatusActive  string
	AccountTokenStatusRevoked string
	AccountTokenStatusExpired string
	TokenCreateTitle          string
	TokenCreateNameLabel      string
	TokenCreateScopesLabel    string
	TokenCreatePresetRead     string
	TokenCreatePresetFull     string
	TokenCreateFooter         string
	TokenShowTitle            string
	TokenShowOnceWarning      string
	TokenShowPrefixLabel      string
	TokenShowScopesLabel      string
	TokenShowSecretLabel      string
	TokenShowCopyHint         string
	TokenShowFooter           string
	TokenRevokedNotice        string
	ErrTokenNameRequired      string
	CheckoutSavedAddresses string
	CheckoutAddressPickHelp string
	CheckoutAddressRePick   string
	AccountFAQ            string
	AccountAbout          string
	AccountFAQBody        string
	AccountAboutBody      string
	AccountFooter         string
	HistoryPaneHelp       string

	// Splash + linking + payment-recovery copy added in the TUI polish batch.
	SplashTagline           string
	ProfileLinkBanner       string
	PaymentExpiredBanner    string
	PaymentExpiredHelp      string
	PaymentRefreshing       string
	CatalogGroupDestacados  string
	CatalogGroupAll         string
	CatalogGroupToggleHint  string
	ProductImageLabel       string
}

// L is the active locale (Spanish Colombia).
var L = SpanishCO()
