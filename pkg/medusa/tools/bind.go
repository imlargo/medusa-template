// Package tools provides utility functions for common operations in Medusa applications.
//
// This package includes helpers for:
//   - Data binding and type conversion
//   - URL parsing and manipulation
//   - Query parameter construction
//
// These utilities help reduce boilerplate and handle edge cases consistently.
package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// MapToStructStrict converts a map to a struct with strict validation.
//
// It ensures that the map keys match the struct fields exactly and does not allow unknown fields.
// If the map contains keys that do not correspond to any field in the struct, an error
// will be returned.
//
// This is useful when you want to ensure that all data in the map can be mapped to the struct
// and there are no unexpected fields.
//
// Example:
//
//	type User struct {
//	    Name  string `json:"name"`
//	    Email string `json:"email"`
//	}
//
//	data := map[string]any{
//	    "name":    "John",
//	    "email":   "john@example.com",
//	    "invalid": "field", // This will cause an error
//	}
//
//	var user User
//	if err := tools.MapToStructStrict(data, &user); err != nil {
//	    log.Fatal(err) // Error: unknown field "invalid"
//	}
//
// Parameters:
//   - data: The map to convert. Keys should match JSON tags in the struct.
//   - result: A pointer to the struct to populate. Must be a pointer to a struct.
//
// Returns an error if:
//   - result is not a pointer to a struct
//   - the map contains unknown fields
//   - JSON marshaling/unmarshaling fails
func MapToStructStrict(data map[string]any, result any) error {
	// Initial checks
	if reflect.TypeOf(result).Kind() != reflect.Ptr {
		return fmt.Errorf("result must be a pointer to struct")
	}

	if reflect.ValueOf(result).Elem().Kind() != reflect.Struct {
		return fmt.Errorf("result must point to a struct")
	}

	// Convert map to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error converting map to JSON: %w", err)
	}

	// Create decoder with strict validation
	decoder := json.NewDecoder(bytes.NewReader(jsonData))
	decoder.DisallowUnknownFields()

	// Convert JSON to struct
	err = decoder.Decode(result)
	if err != nil {
		return fmt.Errorf("error converting JSON to struct (strict mode): %w", err)
	}

	return nil
}

// MapToStruct converts a map to a struct without strict validation.
//
// It allows the map to contain keys that do not correspond to any field in the struct.
// Unknown fields in the map are silently ignored rather than causing an error.
//
// This is useful when you want to extract only the fields you care about from a larger dataset,
// such as partial updates or when working with external APIs that return extra fields.
//
// Example:
//
//	type User struct {
//	    Name  string `json:"name"`
//	    Email string `json:"email"`
//	}
//
//	data := map[string]any{
//	    "name":     "John",
//	    "email":    "john@example.com",
//	    "extra":    "data",    // This will be ignored
//	    "metadata": "ignored", // This will be ignored
//	}
//
//	var user User
//	if err := tools.MapToStruct(data, &user); err != nil {
//	    log.Fatal(err)
//	}
//	// user.Name = "John", user.Email = "john@example.com"
//
// Parameters:
//   - data: The map to convert. Keys should match JSON tags in the struct.
//   - result: A pointer to the struct to populate. Must be a pointer to a struct.
//
// Returns an error if:
//   - result is not a pointer to a struct
//   - JSON marshaling/unmarshaling fails
func MapToStruct(data map[string]any, result any) error {
	// Initial checks
	if reflect.TypeOf(result).Kind() != reflect.Ptr {
		return fmt.Errorf("result must be a pointer to struct")
	}

	if reflect.ValueOf(result).Elem().Kind() != reflect.Struct {
		return fmt.Errorf("result must point to a struct")
	}

	// Convert map to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error converting map to JSON: %w", err)
	}

	// Decode JSON into struct
	err = json.Unmarshal(jsonData, result)
	if err != nil {
		return fmt.Errorf("error converting JSON to struct: %w", err)
	}

	return nil
}
