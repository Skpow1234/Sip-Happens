package email

// ComplianceData is appended to all transactional email templates (minors, SIC, privacy).
type ComplianceData struct {
	MinorAlcoholWarning     string
	PrivacyPolicyURL        string
	ConsumerRightsURL       string
	DataProcessingPolicyURL string
}

func defaultCompliance() ComplianceData {
	return ComplianceData{
		MinorAlcoholWarning:     "Prohibida la venta de alcohol a menores de edad.",
		PrivacyPolicyURL:        "https://sip-happens.local/privacy",
		ConsumerRightsURL:       "https://www.sic.gov.co/proteccion-del-consumidor",
		DataProcessingPolicyURL: "https://sip-happens.local/legal/data-processing",
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
	return d
}
