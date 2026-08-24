package storage

import (
	"fmt"
	"strings"
)

// MaxKeyLength is the longest object key S3 accepts, in UTF-8 bytes.
const MaxKeyLength = 1024

// ValidateKey reports whether key is safe to use as an object key.
//
// A key taken from user input — a filename, a slug — is not automatically
// safe to hand to a provider: nothing in the SDK stops an application from
// sending an empty key, one over the provider's own length limit, one with a
// leading slash that silently addresses a different object than the one
// without it, one with a ".." segment that will resolve to somewhere else
// entirely if the key is ever mirrored onto a real filesystem or served
// through a path-aware CDN, or one carrying a control character that will
// corrupt a log line or a listing the moment it is printed.
//
// ValidateKey rejects all of that before a request is built, so a bad key
// surfaces as [ErrInvalidKey] naming exactly what was wrong, rather than as
// whatever error shape the provider happens to return two calls away from the
// input that caused it.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: key is empty", ErrInvalidKey)
	}
	if len(key) > MaxKeyLength {
		return fmt.Errorf("%w: key is %d bytes, the limit is %d", ErrInvalidKey, len(key), MaxKeyLength)
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: %q starts with '/', which addresses a different key than the one without it", ErrInvalidKey, key)
	}
	for segment := range strings.SplitSeq(key, "/") {
		if segment == "" {
			return fmt.Errorf("%w: %q contains an empty segment (\"//\")", ErrInvalidKey, key)
		}
		if segment == ".." {
			return fmt.Errorf("%w: %q contains a \"..\" segment", ErrInvalidKey, key)
		}
	}
	for _, r := range key {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: %q contains a control character", ErrInvalidKey, key)
		}
	}
	return nil
}
