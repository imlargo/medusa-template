package oauth

// User represents basic user information from any provider
type User struct {
	ID            string         `json:"id"`
	Email         string         `json:"email"`
	Name          string         `json:"name"`
	Picture       string         `json:"picture"`
	VerifiedEmail bool           `json:"verified_email"`
	RawData       map[string]any `json:"raw_data"` // Additional provider-specific data
}

// Provider contains provider-specific configuration
type Provider struct {
	Name         string
	UserInfoURL  string
	Scopes       []string
	FieldMapping FieldMapping
}

// FieldMapping maps provider fields to our generic fields
type FieldMapping struct {
	ID            string
	Email         string
	Name          string
	Picture       string
	VerifiedEmail string
}
