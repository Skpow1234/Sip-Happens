package email

// ComplianceData is appended to all transactional email templates (minors,
// SIC, privacy, data-processing, unsubscribe). Every template embeds this
// via the shared `footer` partial in templates/_footer.{txt,html}; do not
// inline the legal copy into individual templates.
type ComplianceData struct {
	MinorAlcoholWarning     string
	PrivacyPolicyURL        string
	ConsumerRightsURL       string
	DataProcessingPolicyURL string
	// UnsubscribeContact is the email address (or mailto:) the recipient
	// can use to opt out of marketing/shipping update messages. Required
	// for unified compliance footer (DESIGN §5.3 + roadmap 139). Defaults
	// to "soporte@sip-happens.local" if the operator hasn't configured one.
	UnsubscribeContact string
}

func defaultCompliance() ComplianceData {
	return ComplianceData{
		MinorAlcoholWarning:     "Prohibida la venta de alcohol a menores de edad.",
		PrivacyPolicyURL:        "https://sip-happens.local/privacy",
		ConsumerRightsURL:       "https://www.sic.gov.co/proteccion-del-consumidor",
		DataProcessingPolicyURL: "https://sip-happens.local/legal/data-processing",
		UnsubscribeContact:      "soporte@sip-happens.local",
	}
}

func (c ComplianceData) withDefaults() ComplianceData {
	d := defaultCompliance()
	if c.MinorAlcoholWarning != "" {
		d.MinorAlcoholWarning = c.MinorAlcoholWarning
	}
	if c.PrivacyPolicyURL != "" {
		d.PrivacyPolicyURL = c.PrivacyPolicyURL
	}
	if c.ConsumerRightsURL != "" {
		d.ConsumerRightsURL = c.ConsumerRightsURL
	}
	if c.DataProcessingPolicyURL != "" {
		d.DataProcessingPolicyURL = c.DataProcessingPolicyURL
	}
	if c.UnsubscribeContact != "" {
		d.UnsubscribeContact = c.UnsubscribeContact
	}
	return d
}
