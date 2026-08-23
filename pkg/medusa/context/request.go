package context

import (
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// ---------------------------------------------------------------------------
// Binding
// ---------------------------------------------------------------------------

// Bind binds and validates the request body to the given struct.
func (c *Context) Bind(obj any) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		return c.formatValidationError(err)
	}
	return nil
}

// BindQuery binds and validates query parameters.
func (c *Context) BindQuery(obj any) error {
	if err := c.ShouldBindQuery(obj); err != nil {
		return c.formatValidationError(err)
	}
	return nil
}

// BindURI binds and validates URI parameters.
func (c *Context) BindURI(obj any) error {
	if err := c.ShouldBindUri(obj); err != nil {
		return c.formatValidationError(err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Validation error formatting
// ---------------------------------------------------------------------------

// formatValidationError turns a binding failure into a client-facing error:
// a field-by-field map when the validator rejected the value, a plain 400 when
// the payload could not be parsed at all.
func (c *Context) formatValidationError(err error) error {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		details := make(map[string]string, len(validationErrors))
		for _, fieldErr := range validationErrors {
			details[strings.ToLower(fieldErr.Field())] = formatValidationMessage(fieldErr)
		}
		return responses.Validation("validation failed", details)
	}

	return responses.BadRequest("invalid request body")
}

func formatValidationMessage(e validator.FieldError) string {
	kind := e.Type().Kind()

	switch e.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "invalid email format"

	case "min":
		switch kind {
		case reflect.Slice, reflect.Array, reflect.Map:
			return "must have at least " + e.Param() + " elements"
		case reflect.String:
			return "must be at least " + e.Param() + " characters"
		default:
			return "must be at least " + e.Param()
		}

	case "max":
		switch kind {
		case reflect.Slice, reflect.Array, reflect.Map:
			return "must have at most " + e.Param() + " elements"
		case reflect.String:
			return "must be at most " + e.Param() + " characters"
		default:
			return "must be at most " + e.Param()
		}

	case "len":
		switch kind {
		case reflect.Slice, reflect.Array, reflect.Map:
			return "must have exactly " + e.Param() + " elements"
		case reflect.String:
			return "must be exactly " + e.Param() + " characters"
		default:
			return "must have length " + e.Param()
		}

	case "gte":
		return "must be greater than or equal to " + e.Param()
	case "lte":
		return "must be less than or equal to " + e.Param()
	case "gt":
		return "must be greater than " + e.Param()
	case "lt":
		return "must be less than " + e.Param()
	case "eq":
		return "must be equal to " + e.Param()
	case "ne":
		return "must not be equal to " + e.Param()
	case "oneof":
		return "must be one of: " + e.Param()

	case "url":
		return "invalid URL format"
	case "uuid":
		return "invalid UUID format"
	case "uuid4":
		return "invalid UUID v4 format"

	case "alpha":
		return "must contain only letters"
	case "alphanum":
		return "must contain only letters and numbers"
	case "numeric":
		return "must be a valid number"

	case "contains":
		return "must contain '" + e.Param() + "'"
	case "containsany":
		return "must contain at least one of: " + e.Param()
	case "excludes":
		return "must not contain '" + e.Param() + "'"
	case "startswith":
		return "must start with '" + e.Param() + "'"
	case "endswith":
		return "must end with '" + e.Param() + "'"

	case "json":
		return "must be valid JSON"
	case "jwt":
		return "must be a valid JWT token"
	case "datetime":
		return "must be a valid datetime in format: " + e.Param()

	case "ip":
		return "must be a valid IP address"
	case "ipv4":
		return "must be a valid IPv4 address"
	case "ipv6":
		return "must be a valid IPv6 address"

	case "latitude":
		return "must be a valid latitude"
	case "longitude":
		return "must be a valid longitude"

	default:
		return "invalid value"
	}
}

// ---------------------------------------------------------------------------
// URL parameter helpers
// ---------------------------------------------------------------------------

// ParamID gets a URL parameter as uint.
// Returns an error if the parameter is missing, non-numeric, or zero.
func (c *Context) ParamID(name string) (uint, error) {
	param := c.Param(name)
	if param == "" {
		return 0, c.paramError(name + " parameter is required")
	}

	id, err := strconv.ParseUint(param, 10, 64)
	if err != nil {
		return 0, c.paramError("invalid " + name + " parameter: must be a positive integer")
	}
	if id == 0 {
		return 0, c.paramError(name + " parameter must be greater than 0")
	}

	return uint(id), nil
}

// ParamUUID extracts a URL parameter and validates it as a UUID.
// The returned string is always normalized to lowercase via uuid.Parse.
func (c *Context) ParamUUID(name string) (string, error) {
	param := c.Param(name)
	if param == "" {
		return "", c.paramError(name + " parameter is required")
	}

	parsed, err := uuid.Parse(param)
	if err != nil {
		return "", c.paramError("invalid " + name + " parameter: must be a valid UUID")
	}

	// uuid.UUID.String() always returns lowercase, canonical form.
	return parsed.String(), nil
}

// paramError builds a 400 for a malformed URL parameter.
func (c *Context) paramError(message string) error {
	return responses.BadRequest(message)
}

// ---------------------------------------------------------------------------
// Typed query-parameter helpers
// ---------------------------------------------------------------------------

// queryParse is the shared backbone for all typed Query* helpers.
// It reads the named query parameter and parses it with fn.
// On any error (missing or unparseable) it returns defaultValue.
func queryParse[T any](c *Context, name string, defaultValue T, fn func(string) (T, error)) T {
	val := c.Query(name)
	if val == "" {
		return defaultValue
	}
	parsed, err := fn(val)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// QueryInt gets a query parameter as int. Returns defaultValue when the
// parameter is missing or cannot be parsed.
func (c *Context) QueryInt(name string, defaultValue int) int {
	return queryParse(c, name, defaultValue, strconv.Atoi)
}

// QueryInt64 gets a query parameter as int64. Returns defaultValue when the
// parameter is missing or cannot be parsed.
func (c *Context) QueryInt64(name string, defaultValue int64) int64 {
	return queryParse(c, name, defaultValue, func(s string) (int64, error) {
		return strconv.ParseInt(s, 10, 64)
	})
}

// QueryUint gets a query parameter as uint. Returns defaultValue when the
// parameter is missing or cannot be parsed.
func (c *Context) QueryUint(name string, defaultValue uint) uint {
	return queryParse(c, name, defaultValue, func(s string) (uint, error) {
		v, err := strconv.ParseUint(s, 10, 64)
		return uint(v), err
	})
}

// QueryBool gets a query parameter as bool. Returns defaultValue when the
// parameter is missing or cannot be parsed.
func (c *Context) QueryBool(name string, defaultValue bool) bool {
	return queryParse(c, name, defaultValue, strconv.ParseBool)
}

// QueryFloat64 gets a query parameter as float64. Returns defaultValue when
// the parameter is missing or cannot be parsed.
func (c *Context) QueryFloat64(name string, defaultValue float64) float64 {
	return queryParse(c, name, defaultValue, func(s string) (float64, error) {
		return strconv.ParseFloat(s, 64)
	})
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// Pagination constants for consistent default values.
const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
	MinPageSize     = 1
)

// Pagination represents pagination parameters extracted from the query string.
// Validation against MinPageSize / MaxPageSize is enforced both at the binding level
// (via binding tags) and programmatically in GetPageSize to ensure consistency.
type Pagination struct {
	Page     int `form:"page"      binding:"omitempty,min=1"`
	PageSize int `form:"page_size" binding:"omitempty,min=1"`
}

// GetPage returns the validated page number (minimum 1).
func (p Pagination) GetPage() int {
	if p.Page < DefaultPage {
		return DefaultPage
	}
	return p.Page
}

// GetPageSize returns a validated page size clamped to [MinPageSize, MaxPageSize].
// When p.PageSize is not set (< MinPageSize) the provided defaultSize is used,
// itself clamped to the same range.
func (p Pagination) GetPageSize(defaultSize int) int {
	defaultSize = clampPageSize(defaultSize)

	if p.PageSize < MinPageSize {
		return defaultSize
	}
	return clampPageSize(p.PageSize)
}

// Offset calculates the SQL OFFSET for the current page.
func (p Pagination) Offset(defaultSize int) int {
	return (p.GetPage() - 1) * p.GetPageSize(defaultSize)
}

// clampPageSize sanitises a page-size value:
//   - values ≤ 0 (unset / invalid) → DefaultPageSize
//   - values > MaxPageSize          → MaxPageSize
//   - everything else               → unchanged
func clampPageSize(v int) int {
	if v < MinPageSize {
		return DefaultPageSize
	}
	if v > MaxPageSize {
		return MaxPageSize
	}
	return v
}

// Pagination extracts pagination params from the query string and uses
// BindQuery so that struct validation tags are honoured.
func (c *Context) Pagination() (Pagination, error) {
	var p Pagination
	if err := c.BindQuery(&p); err != nil {
		return Pagination{}, err
	}
	return p, nil
}

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

// SortOrder represents the sorting direction.
type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// SortParams holds the resolved sorting field and direction.
type SortParams struct {
	Field string
	Order SortOrder
}

// Sort extracts sorting parameters from the query string.
// allowedFields is a whitelist of column names the caller permits. Any
// sort_by value not in the set falls back to defaultField.
// If allowedFields is non-empty, defaultField must itself be in the list —
// if it is not, the first allowed field is used as the fallback instead.
//
// Example: ?sort_by=created_at&sort_order=desc
func (c *Context) Sort(defaultField string, defaultOrder SortOrder, allowedFields ...string) SortParams {
	// Resolve a safe default: if the caller's default is not in the whitelist,
	// fall back to the first allowed field to avoid silently using an invalid column.
	safeDefault := resolveDefault(defaultField, allowedFields)

	field := c.Query("sort_by")
	if !isAllowedField(field, allowedFields) {
		field = safeDefault
	}

	order := SortOrder(c.Query("sort_order"))
	if order != SortOrderAsc && order != SortOrderDesc {
		order = defaultOrder
	}

	return SortParams{
		Field: field,
		Order: order,
	}
}

// resolveDefault guarantees the returned default is safe to use.
// If allowed is empty the original default passes through (no whitelist = no
// restriction, caller's responsibility). When a whitelist exists, default must
// appear in it; otherwise we fall back to allowed[0].
func resolveDefault(defaultField string, allowed []string) string {
	if len(allowed) == 0 {
		return defaultField
	}
	for _, a := range allowed {
		if a == defaultField {
			return defaultField
		}
	}
	return allowed[0]
}

// isAllowedField reports whether candidate is a permitted sort field.
// An empty or nil whitelist means no restriction (any non-empty value passes).
// When a whitelist is provided, candidate must be present in it.
func isAllowedField(candidate string, allowed []string) bool {
	if candidate == "" {
		return false
	}
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if candidate == a {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Request metadata
// ---------------------------------------------------------------------------

// ClientIP returns the real client IP address.
func (c *Context) ClientIP() string {
	return c.Context.ClientIP()
}

// BearerToken extracts and trims the token from the Authorization header.
// Returns an error when the header is missing or malformed.
func (c *Context) BearerToken() (string, error) {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return "", errors.New("authorization header is missing")
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("authorization header must be in format 'Bearer token'")
	}

	return strings.TrimSpace(parts[1]), nil
}
