// pkg/medusa/context_auth.go
package context

// UserID returns the authenticated user's ID.
func (c *Context) UserID() (uint, bool) {
	id, exists := c.Get(UserIDContextKey)
	if !exists {
		return 0, false
	}
	switch v := id.(type) {
	case uint:
		return v, true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	case float64:
		return uint(v), true
	default:
		return 0, false
	}
}

// MustUserID returns the user ID or panics if not authenticated.
// WARNING: This will be caught by recovery middleware and result in a 500 error.
// Use UserID() and handle the bool return for proper 401 Unauthorized responses.
func (c *Context) MustUserID() uint {
	id, ok := c.UserID()
	if !ok {
		panic("medusa: user not authenticated")
	}
	return id
}

// IsAuthenticated returns true if user is authenticated.
func (c *Context) IsAuthenticated() bool {
	_, exists := c.UserID()
	return exists
}

// SetUserID sets the authenticated user ID (called by auth middleware).
func (c *Context) SetUserID(id uint) {
	c.Set(UserIDContextKey, id)
}
