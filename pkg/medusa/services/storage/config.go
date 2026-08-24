package storage

import (
	"fmt"
	"strings"
	"time"
)

// DefaultPresignExpiry is what [FileStorage.GetPresignedURL] uses when the
// caller passes a non-positive expiry.
const DefaultPresignExpiry = 15 * time.Minute

// MaxPresignExpiry is the longest expiry a presigned URL may carry. This is
// not this package's opinion: SigV4 presigning cannot exceed seven days,
// because the signature itself expires, not just an application's idea of
// how long the link should work.
const MaxPresignExpiry = 7 * 24 * time.Hour

// Config holds the credentials and addressing needed to reach a bucket.
//
// The zero Config is not usable — BucketName, AccountID, AccessKeyID and
// SecretAccessKey are all required. [Config.Validate] reports every problem
// at once, so a deployment missing two of the four fails with both named
// rather than with the first one found.
type Config struct {
	BucketName      string
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string

	// PublicDomain serves [FileStorage.GetPublicURL] links from a custom
	// domain instead of the provider's own public endpoint. Optional.
	PublicDomain string

	// UsePublicURL makes Upload populate FileResult.URL. Off by default: a
	// bucket is private unless an application has actually put a domain in
	// front of it, and defaulting to on would hand out links that 403.
	UsePublicURL bool
}

// Validate reports every missing required field at once.
func (c Config) Validate() error {
	var missing []string
	if c.BucketName == "" {
		missing = append(missing, "BucketName")
	}
	if c.AccountID == "" {
		missing = append(missing, "AccountID")
	}
	if c.AccessKeyID == "" {
		missing = append(missing, "AccessKeyID")
	}
	if c.SecretAccessKey == "" {
		missing = append(missing, "SecretAccessKey")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %s", ErrInvalidConfig, strings.Join(missing, ", "))
	}
	return nil
}
