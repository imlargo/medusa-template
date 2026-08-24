// Package storage provides object storage for application files: upload,
// download, delete, and the two ways of handing a client a link to an
// object.
//
// # Getting started
//
//	fs, err := storage.NewFileStorage(ctx, storage.ProviderR2, storage.Config{
//	    BucketName:      "assets",
//	    AccountID:       accountID,
//	    AccessKeyID:     accessKeyID,
//	    SecretAccessKey: secretAccessKey,
//	})
//	if err != nil {
//	    return err
//	}
//
//	result, err := fs.Upload(ctx, "avatars/42.png", reader, "image/png", size)
//
// # Fails at construction, not on first use
//
// [NewFileStorage] confirms the bucket is reachable with the credentials it
// was given before it returns. A typo'd bucket name or a revoked key is then
// a startup error naming the problem, rather than a failure surfacing an hour
// into production on whichever request happens to upload first.
//
// # Keys are validated before they are sent
//
// Every method that takes a key runs it through [ValidateKey] first. A key
// built from user input — a filename, a slug — is not automatically safe to
// hand to a provider: nothing in the SDK stops an application from sending an
// empty key, one with a ".." segment, or one carrying a control character
// that will corrupt a log line or a listing later. Rejecting those before a
// request is sent turns a provider-specific failure two calls away from its
// cause into [ErrInvalidKey] naming exactly what was wrong.
//
// # Deletion reports partial failure
//
// [FileStorage.BulkDelete] returns a [DeleteResult] listing which keys were
// removed and which were not, rather than one combined error. A thousand-key
// batch where three keys are denied by a bucket policy is not "an error"; it
// is 997 successes and three failures a caller can retry individually,
// and folding that into a single error string would throw the distinction
// away.
//
// # What it will not do
//
// This is not a general-purpose blob abstraction over every provider that
// speaks a different protocol: it targets S3-compatible object storage.
// Providers are added deliberately — see [Provider] — because building a
// client for one that was never tested against this package is precisely the
// kind of failure this package exists to surface at startup instead of on
// first use.
package storage
