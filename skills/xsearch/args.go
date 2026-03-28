package xsearch

// Typed argument structs for X search tools.

type SearchArgs struct {
	Query   string   `json:"query" jsonschema:"Search query -- what to search for on X (e.g. 'AI news today', 'latest from @elonmusk')"`
	Handles []string `json:"handles,omitempty" jsonschema:"Filter to specific X handles (without @, max 10). Cannot be used with exclude_handles."`
	ExcludeHandles []string `json:"exclude_handles,omitempty" jsonschema:"Exclude specific X handles (without @, max 10). Cannot be used with handles."`
	FromDate string `json:"from_date,omitempty" jsonschema:"Start date filter, YYYY-MM-DD format"`
	ToDate   string `json:"to_date,omitempty" jsonschema:"End date filter, YYYY-MM-DD format"`
}

type SearchWithMediaArgs struct {
	Query          string   `json:"query" jsonschema:"Search query -- what to search for on X"`
	Handles        []string `json:"handles,omitempty" jsonschema:"Filter to specific X handles (without @, max 10)"`
	ExcludeHandles []string `json:"exclude_handles,omitempty" jsonschema:"Exclude specific X handles (without @, max 10)"`
	FromDate       string   `json:"from_date,omitempty" jsonschema:"Start date filter, YYYY-MM-DD format"`
	ToDate         string   `json:"to_date,omitempty" jsonschema:"End date filter, YYYY-MM-DD format"`
	Images         bool     `json:"images,omitempty" jsonschema:"Enable image understanding -- analyze images in posts"`
	Video          bool     `json:"video,omitempty" jsonschema:"Enable video understanding -- analyze video content in posts"`
}

type ProfileSearchArgs struct {
	Handle   string `json:"handle" jsonschema:"X handle to search (without @)"`
	Query    string `json:"query,omitempty" jsonschema:"Optional topic filter for this user's posts"`
	FromDate string `json:"from_date,omitempty" jsonschema:"Start date filter, YYYY-MM-DD format"`
	ToDate   string `json:"to_date,omitempty" jsonschema:"End date filter, YYYY-MM-DD format"`
}
