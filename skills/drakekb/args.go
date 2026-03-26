package drakekb

// Typed argument structs for Drake KB tools.

type EmptyArgs struct{}

type SearchArgs struct {
	Query    string `json:"query" jsonschema:"Search query (e.g. Form 2441, dependent care credit, EF message)"`
	Category string `json:"category,omitempty" jsonschema:"Filter by category: Drake-Tax, Resources, DAS, Banking, Broadcasts"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results, default 10"`
}

type ReadArticleArgs struct {
	URL string `json:"url" jsonschema:"Full URL of the Drake KB article to read"`
}

type FormLookupArgs struct {
	FormNumber string `json:"form_number" jsonschema:"IRS form number (e.g. 2441, 1040, W-2, Schedule C)"`
}
