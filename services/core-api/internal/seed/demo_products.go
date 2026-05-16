package seed

type demoProduct struct {
	SKU         string
	Name        string
	Brewery     string
	Style       string
	ABV         float64
	VolumeML    int
	Invima      string
	Description string
	IBU         *int
	PriceCOP    int64
	OnHand      int
}

// demoProducts returns the catalog used for local/demo stacks (20 SKUs for pagination tests).
func demoProducts() []demoProduct {
	return []demoProduct{
		{
			SKU: "DEMO-IPA-330", Name: "Demo IPA 330ml", Brewery: "Demo Brewery", Style: "IPA",
			ABV: 6.2, VolumeML: 330, Invima: "INVIMA-DEMO-IPA-001", PriceCOP: 11900, OnHand: 50,
			Description: "IPA cítrica de prueba con notas a pomelo.", IBU: intPtr(45),
		},
		{
			SKU: "DEMO-LAGER-330", Name: "Demo Lager 330ml", Brewery: "Demo Brewery", Style: "Lager",
			ABV: 4.6, VolumeML: 330, Invima: "INVIMA-DEMO-LAGER-001", PriceCOP: 8900, OnHand: 80,
			Description: "Lager suave y refrescante.", IBU: intPtr(18),
		},
		{
			SKU: "BOG-PILS-330", Name: "Pilsner Sabana 330ml", Brewery: "Cerveza Sabana", Style: "Pilsner",
			ABV: 4.8, VolumeML: 330, Invima: "INVIMA-BOG-PILS-001", PriceCOP: 9900, OnHand: 60,
			Description: "Pilsner crujiente, final seco.", IBU: intPtr(22),
		},
		{
			SKU: "AND-RED-330", Name: "Red Ale Cordillera 330ml", Brewery: "Andina Brew Co", Style: "Red Ale",
			ABV: 5.4, VolumeML: 330, Invima: "INVIMA-AND-RED-001", PriceCOP: 10900, OnHand: 45,
			Description: "Caramelo suave y lúpulo balanceado.", IBU: intPtr(28),
		},
		{
			SKU: "VAL-STOUT-330", Name: "Stout del Valle 330ml", Brewery: "Valle Oscuro", Style: "Stout",
			ABV: 6.0, VolumeML: 330, Invima: "INVIMA-VAL-STOUT-001", PriceCOP: 12900, OnHand: 35,
			Description: "Chocolate y café tostado.", IBU: intPtr(35),
		},
		{
			SKU: "CAR-IPA-473", Name: "IPA Caribe 473ml", Brewery: "Caribe Hops", Style: "IPA",
			ABV: 6.8, VolumeML: 473, Invima: "INVIMA-CAR-IPA-001", PriceCOP: 14900, OnHand: 40,
			Description: "IPA tropical en lata grande.", IBU: intPtr(55),
		},
		{
			SKU: "PAC-WHEAT-330", Name: "Wheat Pacífico 330ml", Brewery: "Pacífico Brewing", Style: "Wheat",
			ABV: 4.9, VolumeML: 330, Invima: "INVIMA-PAC-WHEAT-001", PriceCOP: 10500, OnHand: 55,
			Description: "Trigo con notas a plátano y clavo.", IBU: intPtr(12),
		},
		{
			SKU: "SAB-PORTER-330", Name: "Porter Sabana 330ml", Brewery: "Cerveza Sabana", Style: "Porter",
			ABV: 5.6, VolumeML: 330, Invima: "INVIMA-SAB-PORTER-001", PriceCOP: 11500, OnHand: 30,
			Description: "Porter robusta, final a regaliz.", IBU: intPtr(32),
		},
		{
			SKU: "CORD-BLONDE-330", Name: "Blonde Cordillera 330ml", Brewery: "Andina Brew Co", Style: "Blonde",
			ABV: 5.0, VolumeML: 330, Invima: "INVIMA-CORD-BLONDE-001", PriceCOP: 9500, OnHand: 70,
			Description: "Rubia ligera para tardes soleadas.", IBU: intPtr(20),
		},
		{
			SKU: "META-AMBER-330", Name: "Ámbar Meta 330ml", Brewery: "Llanos Craft", Style: "Amber Ale",
			ABV: 5.5, VolumeML: 330, Invima: "INVIMA-META-AMBER-001", PriceCOP: 10200, OnHand: 48,
			Description: "Ámbar con miel de yucá.", IBU: intPtr(26),
		},
		{
			SKU: "CALI-SOUR-330", Name: "Sour Cítrica Cali 330ml", Brewery: "Valle Fermentos", Style: "Sour",
			ABV: 4.5, VolumeML: 330, Invima: "INVIMA-CALI-SOUR-001", PriceCOP: 13500, OnHand: 25,
			Description: "Sour ácida con maracuyá.", IBU: intPtr(8),
		},
		{
			SKU: "MED-SESSION-330", Name: "Session IPA Medellín 330ml", Brewery: "Paisa Hops", Style: "Session IPA",
			ABV: 4.2, VolumeML: 330, Invima: "INVIMA-MED-SESSION-001", PriceCOP: 10800, OnHand: 65,
			Description: "IPA de sesión, amarga pero bebible.", IBU: intPtr(38),
		},
		{
			SKU: "EJE-DOUBLE-330", Name: "Double IPA Eje 330ml", Brewery: "Eje Central", Style: "Double IPA",
			ABV: 8.2, VolumeML: 330, Invima: "INVIMA-EJE-DOUBLE-001", PriceCOP: 15900, OnHand: 20,
			Description: "Doble lúpulo, resina y pino.", IBU: intPtr(72),
		},
		{
			SKU: "LLA-LAGER-473", Name: "Lager Llanera 473ml", Brewery: "Llanos Craft", Style: "Lager",
			ABV: 4.7, VolumeML: 473, Invima: "INVIMA-LLA-LAGER-001", PriceCOP: 11200, OnHand: 90,
			Description: "Lager en botella grande para compartir.", IBU: intPtr(16),
		},
		{
			SKU: "ANT-MUNICH-330", Name: "Munich Antioquia 330ml", Brewery: "Paisa Hops", Style: "Munich",
			ABV: 5.3, VolumeML: 330, Invima: "INVIMA-ANT-MUNICH-001", PriceCOP: 10100, OnHand: 42,
			Description: "Malta Munich, pan tostado.", IBU: intPtr(24),
		},
		{
			SKU: "ORI-KOLSCH-330", Name: "Kölsch Orinoquía 330ml", Brewery: "Orinoquía Brew", Style: "Kölsch",
			ABV: 4.9, VolumeML: 330, Invima: "INVIMA-ORI-KOLSCH-001", PriceCOP: 9800, OnHand: 38,
			Description: "Fermentada fría, floral y limpia.", IBU: intPtr(19),
		},
		{
			SKU: "CAF-STOUT-330", Name: "Stout Café Sierra 330ml", Brewery: "Sierra Nevada Brew", Style: "Coffee Stout",
			ABV: 6.4, VolumeML: 330, Invima: "INVIMA-CAF-STOUT-001", PriceCOP: 13800, OnHand: 28,
			Description: "Stout con café colombiano de origen.", IBU: intPtr(30),
		},
		{
			SKU: "TRO-TROPICAL-330", Name: "Tropical Hazy 330ml", Brewery: "Caribe Hops", Style: "Hazy IPA",
			ABV: 6.1, VolumeML: 330, Invima: "INVIMA-TRO-TROPICAL-001", PriceCOP: 14200, OnHand: 33,
			Description: "Neipa turbia, mango y papaya.", IBU: intPtr(42),
		},
		{
			SKU: "BQB-BARLEY-330", Name: "Barleywine Boyacá 330ml", Brewery: "Altiplano Cerveza", Style: "Barleywine",
			ABV: 9.0, VolumeML: 330, Invima: "INVIMA-BQB-BARLEY-001", PriceCOP: 18900, OnHand: 15,
			Description: "Barleywine de guarda, frutos secos.", IBU: intPtr(48),
		},
		{
			SKU: "FIN-BELGIAN-330", Name: "Belga Fuerte 330ml", Brewery: "Valle Oscuro", Style: "Belgian Strong",
			ABV: 7.5, VolumeML: 330, Invima: "INVIMA-FIN-BELGIAN-001", PriceCOP: 16500, OnHand: 22,
			Description: "Belga especiada, final etílico suave.", IBU: intPtr(26),
		},
	}
}

func intPtr(n int) *int { return &n }
