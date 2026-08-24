package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// FileStorage is the operations an application needs against an object
// store: write, read, remove, and the two ways of handing a client a link.
//
// Every method takes a context and is safe for concurrent use. Every method
// that takes a key validates it with [ValidateKey] before using it.
type FileStorage interface {
	// Upload writes reader to key. size must be the exact number of bytes
	// reader will produce; providers reject a mismatch rather than guess.
	Upload(ctx context.Context, key string, reader io.Reader, contentType string, size int64) (*FileResult, error)

	// Download returns the object's content. The caller must close it.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// GetFileForDownload is Download plus the metadata a caller typically
	// needs to serve the object onward: its size and content type.
	GetFileForDownload(ctx context.Context, key string) (*FileDownload, error)

	// Delete removes one object. Deleting a key that does not exist is not
	// an error.
	Delete(ctx context.Context, key string) error

	// BulkDelete removes many objects, splitting the request into batches of
	// [MaxBatchSize]. It reports which keys were removed and which were not,
	// rather than stopping at the first failure or the first batch's error.
	//
	// The returned *DeleteResult is never nil once any batch has run, even
	// when err is non-nil: it carries every key resolved by the batches that
	// completed before the one that failed, so a batch request itself
	// failing outright does not erase the record of what earlier batches
	// already deleted.
	BulkDelete(ctx context.Context, keys []string) (*DeleteResult, error)

	// GetPresignedURL returns a URL granting temporary access to key without
	// the caller's own credentials. expiry is clamped to
	// [DefaultPresignExpiry] when non-positive, and rejected outright past
	// [MaxPresignExpiry].
	GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)

	// GetPublicURL returns the URL at which key is served if the bucket (or
	// [Config.PublicDomain]) makes it publicly readable. It does not check
	// that the object exists or that the bucket is actually public — it
	// builds the URL the provider's own rules imply.
	GetPublicURL(key string) (string, error)
}

// s3API is the subset of *s3.Client this package calls. Depending on this
// rather than on *s3.Client directly is what makes fileStorage testable
// without a network, and incidentally makes it usable against anything
// wire-compatible with these five calls.
type s3API interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	DeleteObjects(ctx context.Context, in *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	HeadBucket(ctx context.Context, in *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

// presignAPI is the one presigning call this package uses. It is separate
// from s3API because *s3.PresignClient, unlike the calls above, is not
// something a fake can implement by satisfying *s3.Client's own shape — the
// real SDK wraps a client to build one.
type presignAPI interface {
	PresignGetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4signer.PresignedHTTPRequest, error)
}

type fileStorage struct {
	client    s3API
	presigner presignAPI
	provider  Provider
	config    Config
}

// NewFileStorage builds a [FileStorage] for provider, using cfg's
// credentials.
//
// It confirms the bucket is reachable — a HeadBucket call bounded by an
// internal timeout — before returning, so a typo'd bucket name or a revoked
// key is a startup error naming the problem, not a failure surfacing on
// whichever request happens to touch storage first. ctx bounds only that
// check; it is not retained past this call.
//
// The credential therefore needs HeadBucket permission on the bucket, in
// addition to whatever object-level actions the application performs. A
// credential scoped to only PutObject/GetObject/DeleteObject on a prefix —
// a common least-privilege pattern — will fail here even though every
// operation this package actually performs would have worked; grant
// HeadBucket (or ListBucket, which implies it on most providers) alongside
// them.
func NewFileStorage(ctx context.Context, provider Provider, cfg Config) (FileStorage, error) {
	if !provider.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedProvider, provider)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newClient(ctx, provider, cfg)
	if err != nil {
		return nil, fmt.Errorf("storage: build client: %w", err)
	}

	checkCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if _, err := client.HeadBucket(checkCtx, &s3.HeadBucketInput{Bucket: aws.String(cfg.BucketName)}); err != nil {
		return nil, fmt.Errorf("storage: bucket %q is not reachable: %w", cfg.BucketName, err)
	}

	return newFileStorage(client, s3.NewPresignClient(client), provider, cfg), nil
}

// newFileStorage wires a FileStorage from an already-built client. Split out
// from NewFileStorage so tests can exercise every operation against a fake
// s3API and presignAPI without a network.
//
// provider must be present in the providers registry — NewFileStorage
// guarantees this via Provider.IsValid() before it ever calls this function.
// A test that wants to exercise a provider must register it in providers
// first; passing one that is not there makes publicURL panic on a nil
// map entry rather than return an error, since that path is unreachable
// through the public API and is not worth an error return every caller has
// to check for a case that can only be reached by breaking this precondition.
func newFileStorage(client s3API, presigner presignAPI, provider Provider, cfg Config) *fileStorage {
	return &fileStorage{client: client, presigner: presigner, provider: provider, config: cfg}
}

func (s *fileStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string, size int64) (*FileResult, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	out, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.config.BucketName),
		Key:           aws.String(key),
		Body:          reader,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: upload %q: %w", key, err)
	}

	result := &FileResult{
		Key:         key,
		Size:        size,
		ContentType: contentType,
		ETag:        strings.Trim(aws.ToString(out.ETag), `"`),
	}

	if s.config.UsePublicURL {
		// publicURL, not GetPublicURL: key is already known valid, and the
		// object is already written. A public-URL builder failing at this
		// point would report a successful write as a failed Upload — worse
		// than the URL it can't build — and publicURL cannot fail: it only
		// ever runs for a provider the providers registry has a builder for.
		result.URL = s.publicURL(key)
	}

	return result, nil
}

// getObject validates key and fetches the object, wrapping any client error
// once — the one place Download and GetFileForDownload's shared request
// belongs, so a change to either has nowhere else to be applied.
func (s *fileStorage) getObject(ctx context.Context, key string) (*s3.GetObjectOutput, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: download %q: %w", key, err)
	}
	return out, nil
}

func (s *fileStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.getObject(ctx, key)
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *fileStorage) GetFileForDownload(ctx context.Context, key string) (*FileDownload, error) {
	out, err := s.getObject(ctx, key)
	if err != nil {
		return nil, err
	}

	contentType := "application/octet-stream"
	if out.ContentType != nil {
		contentType = *out.ContentType
	}

	return &FileDownload{
		Content:     out.Body,
		ContentType: contentType,
		Size:        aws.ToInt64(out.ContentLength),
	}, nil
}

func (s *fileStorage) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}

	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("storage: delete %q: %w", key, err)
	}

	return nil
}

func (s *fileStorage) BulkDelete(ctx context.Context, keys []string) (*DeleteResult, error) {
	if len(keys) == 0 {
		return &DeleteResult{}, nil
	}
	for _, key := range keys {
		if err := ValidateKey(key); err != nil {
			return nil, err
		}
	}

	result := &DeleteResult{}
	for i := 0; i < len(keys); i += MaxBatchSize {
		end := min(i+MaxBatchSize, len(keys))

		deleted, failed, err := s.deleteBatch(ctx, keys[i:end])
		result.Deleted = append(result.Deleted, deleted...)
		result.Failed = append(result.Failed, failed...)
		if err != nil {
			// result still carries every batch that succeeded before this
			// one, so a caller does not lose track of what was actually
			// deleted just because a later batch's request failed outright.
			return result, fmt.Errorf("storage: bulk delete batch %d-%d: %w", i, end-1, err)
		}
	}

	return result, nil
}

// deleteBatch deletes a single batch of at most [MaxBatchSize] keys. The
// batch call itself failing (a network error, a denied bucket) is returned
// as an error; a specific key within it being denied is reported in the
// returned failures instead, because the batch as a whole still succeeded.
func (s *fileStorage) deleteBatch(ctx context.Context, keys []string) ([]string, []DeleteFailure, error) {
	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(key)}
	}

	out, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(s.config.BucketName),
		Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(false)},
	})
	if err != nil {
		return nil, nil, err
	}

	deleted := make([]string, 0, len(out.Deleted))
	for _, d := range out.Deleted {
		deleted = append(deleted, aws.ToString(d.Key))
	}

	failed := make([]DeleteFailure, 0, len(out.Errors))
	for _, e := range out.Errors {
		failed = append(failed, DeleteFailure{Key: aws.ToString(e.Key), Reason: aws.ToString(e.Message)})
	}

	return deleted, failed, nil
}

func (s *fileStorage) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	switch {
	case expiry <= 0:
		expiry = DefaultPresignExpiry
	case expiry > MaxPresignExpiry:
		return "", fmt.Errorf("storage: requested expiry %v exceeds the %v a presigned URL can carry", expiry, MaxPresignExpiry)
	}

	out, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(key),
	}, func(o *s3.PresignOptions) { o.Expires = expiry })
	if err != nil {
		return "", fmt.Errorf("storage: presign %q: %w", key, err)
	}

	return out.URL, nil
}

func (s *fileStorage) GetPublicURL(key string) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	return s.publicURL(key), nil
}

// publicURL builds the public URL for key without validating it again — the
// exported GetPublicURL already has, and so has Upload by the time it calls
// this after a successful write. It cannot fail: newFileStorage is only ever
// handed a provider present in the providers registry, and every entry there
// has a publicURL builder.
func (s *fileStorage) publicURL(key string) string {
	escapedKey := (&url.URL{Path: key}).EscapedPath()

	if s.config.PublicDomain != "" {
		return fmt.Sprintf("https://%s/%s", s.config.PublicDomain, escapedKey)
	}
	return providers[s.provider].publicURL(s.config, escapedKey)
}
