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
		publicURL, err := s.GetPublicURL(key)
		if err != nil {
			return nil, err
		}
		result.URL = publicURL
	}

	return result, nil
}

func (s *fileStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
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

	return out.Body, nil
}

func (s *fileStorage) GetFileForDownload(ctx context.Context, key string) (*FileDownload, error) {
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
		if err != nil {
			return nil, fmt.Errorf("storage: bulk delete batch %d-%d: %w", i, end-1, err)
		}
		result.Deleted = append(result.Deleted, deleted...)
		result.Failed = append(result.Failed, failed...)
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
	escapedKey := escapeKeyPath(key)

	if s.config.PublicDomain != "" {
		return fmt.Sprintf("https://%s/%s", s.config.PublicDomain, escapedKey), nil
	}

	switch s.provider {
	case ProviderR2:
		return fmt.Sprintf("https://pub-%s.r2.dev/%s", s.config.AccountID, escapedKey), nil
	default:
		return "", fmt.Errorf("%w: %q has no public URL scheme", ErrUnsupportedProvider, s.provider)
	}
}

// escapeKeyPath percent-encodes each "/"-separated segment of key, so a key
// containing a space, '#', '?' or other character reserved in a URL still
// produces a link that resolves to it, instead of one truncated at the first
// such character or that resolves to a different object entirely.
func escapeKeyPath(key string) string {
	segments := strings.Split(key, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
