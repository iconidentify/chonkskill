package fredmeyer

// Typed argument structs for each tool. JSON tags control the parameter names,
// jsonschema tags provide descriptions for the generated schema.

type EmptyArgs struct{}

type SearchLocationsArgs struct {
	ZipCode     string `json:"zip_code" jsonschema:"ZIP code to search near"`
	RadiusMiles int    `json:"radius_miles,omitempty" jsonschema:"Search radius in miles, 1-100, default 10"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Max results, 1-200, default 10"`
	Chain       string `json:"chain,omitempty" jsonschema:"Filter by chain name (e.g. FRED MEYER STORES, QFC, KROGER)"`
}

type LocationIDArgs struct {
	LocationID string `json:"location_id" jsonschema:"The Kroger location ID (e.g. 70100023)"`
}

type SearchProductsArgs struct {
	Term        string `json:"term" jsonschema:"Search term (e.g. organic whole milk, honeycrisp apples)"`
	LocationID  string `json:"location_id,omitempty" jsonschema:"Store location ID for store-specific pricing and availability"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Max results, 1-50, default 10"`
	Brand       string `json:"brand,omitempty" jsonschema:"Filter by brand name (exact match)"`
	Fulfillment string `json:"fulfillment,omitempty" jsonschema:"Filter by fulfillment type: ais (in-store), csp (pickup), dth (delivery)"`
}

type ProductDetailsArgs struct {
	ProductID  string `json:"product_id" jsonschema:"The product ID or UPC code"`
	LocationID string `json:"location_id,omitempty" jsonschema:"Store location ID for pricing"`
}

type AddToCartArgs struct {
	UPC       string `json:"upc" jsonschema:"Product UPC code from search results"`
	Quantity  int    `json:"quantity,omitempty" jsonschema:"Quantity to add, default 1"`
	ProductID string `json:"product_id,omitempty" jsonschema:"Product ID for local tracking"`
	Name      string `json:"name,omitempty" jsonschema:"Product name for local cart display"`
	Modality  string `json:"modality,omitempty" jsonschema:"Fulfillment type: PICKUP or DELIVERY, default PICKUP"`
}

type RemoveFromCartArgs struct {
	ProductID string `json:"product_id" jsonschema:"Product ID to remove from local tracking"`
	Modality  string `json:"modality,omitempty" jsonschema:"Fulfillment type to match, removes all if omitted"`
}

type ViewOrderHistoryArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"Number of recent orders to show, default 10"`
}

type RedirectURLArgs struct {
	RedirectURL string `json:"redirect_url" jsonschema:"The full redirect URL from the browser after Kroger authorization"`
}
