package storage

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateKey(t *testing.T) {
	tests := map[string]struct {
		key     string
		wantErr bool
	}{
		"simple key":         {key: "avatars/42.png", wantErr: false},
		"nested key":         {key: "a/b/c/d.txt", wantErr: false},
		"single segment":     {key: "readme.txt", wantErr: false},
		"empty":              {key: "", wantErr: true},
		"leading slash":      {key: "/avatars/42.png", wantErr: true},
		"double slash":       {key: "avatars//42.png", wantErr: true},
		"dot-dot segment":    {key: "a/../b", wantErr: true},
		"dot-dot alone":      {key: "..", wantErr: true},
		"dot-dot substring":  {key: "notes..txt", wantErr: false}, // not a distinct segment
		"control character":  {key: "avatars/42\n.png", wantErr: true},
		"tab":                {key: "avatars/42\t.png", wantErr: true},
		"too long":           {key: strings.Repeat("a", MaxKeyLength+1), wantErr: true},
		"exactly max length": {key: strings.Repeat("a", MaxKeyLength), wantErr: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateKey(tc.key)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateKey(%q) = nil, want an error", tc.key)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateKey(%q) = %v, want nil", tc.key, err)
			}
			if tc.wantErr && !errors.Is(err, ErrInvalidKey) {
				t.Errorf("ValidateKey(%q) = %v, want it to wrap ErrInvalidKey", tc.key, err)
			}
		})
	}
}
