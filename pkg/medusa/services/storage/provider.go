package storage

// Provider identifies which object storage service a [Config]'s credentials
// belong to, and therefore which client construction and public-URL rules
// apply.
//
// The set is closed rather than an open string an application can extend
// from outside the package: adding one means adding the client construction
// and public-URL code to go with it, in this package, where both can be
// tested against the provider they claim to support.
type Provider string

// ProviderR2 is Cloudflare R2, an S3-compatible object store.
const ProviderR2 Provider = "r2"

// IsValid reports whether p is a [Provider] this package can build a client
// for.
func (p Provider) IsValid() bool {
	switch p {
	case ProviderR2:
		return true
	default:
		return false
	}
}
