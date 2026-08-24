package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Provider identifies which object storage service a [Config]'s credentials
// belong to, and therefore which client construction and public-URL rules
// apply.
//
// The set is closed rather than an open string an application can extend
// from outside the package: adding one means adding an entry to the
// providers registry below, in this package, where the client it builds and
// the URL it constructs can both be tested against the provider they claim
// to support.
type Provider string

// ProviderR2 is Cloudflare R2, an S3-compatible object store.
const ProviderR2 Provider = "r2"

// providerSpec is everything this package knows about one [Provider]: how to
// build a client for it, and how to build a public URL for one of its keys.
//
// Tying both to the same registry entry is deliberate. Two provider switches
// that happen to list the same cases only by discipline drift apart the
// moment a new provider is added to one and forgotten in the other — this
// registry makes "the provider builds a client" and "the provider builds a
// public URL" the same fact, checked once, at [Provider.IsValid].
type providerSpec struct {
	newClient func(ctx context.Context, cfg Config) (*s3.Client, error)
	publicURL func(cfg Config, escapedKey string) string
}

var providers = map[Provider]providerSpec{
	ProviderR2: {
		newClient: newR2Client,
		publicURL: func(cfg Config, escapedKey string) string {
			return fmt.Sprintf("https://pub-%s.r2.dev/%s", cfg.AccountID, escapedKey)
		},
	},
}

// IsValid reports whether p is a [Provider] this package can build a client
// and a public URL for.
func (p Provider) IsValid() bool {
	_, ok := providers[p]
	return ok
}
