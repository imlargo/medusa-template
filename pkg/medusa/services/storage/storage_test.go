package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeS3 is a hand-rolled s3API: no network, and every call records what it
// was given so a test can assert on the request as well as the response.
type fakeS3 struct {
	putIn  *s3.PutObjectInput
	putOut *s3.PutObjectOutput
	putErr error

	getOut *s3.GetObjectOutput
	getErr error

	deleteIn  *s3.DeleteObjectInput
	deleteErr error

	deleteObjectsIn  []*s3.DeleteObjectsInput // one per call, so batching is observable
	deleteObjectsOut []*s3.DeleteObjectsOutput
	deleteObjectsErr error

	headErr error
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.putIn = in
	if f.putErr != nil {
		return nil, f.putErr
	}
	if f.putOut != nil {
		return f.putOut, nil
	}
	return &s3.PutObjectOutput{ETag: aws.String(`"deadbeef"`)}, nil
}

func (f *fakeS3) GetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getOut, nil
}

func (f *fakeS3) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.deleteIn = in
	return &s3.DeleteObjectOutput{}, f.deleteErr
}

func (f *fakeS3) DeleteObjects(_ context.Context, in *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	f.deleteObjectsIn = append(f.deleteObjectsIn, in)
	if f.deleteObjectsErr != nil {
		return nil, f.deleteObjectsErr
	}
	if len(f.deleteObjectsOut) > 0 {
		out := f.deleteObjectsOut[0]
		f.deleteObjectsOut = f.deleteObjectsOut[1:]
		return out, nil
	}
	// Default: everything in the batch succeeded.
	deleted := make([]types.DeletedObject, len(in.Delete.Objects))
	for i, obj := range in.Delete.Objects {
		deleted[i] = types.DeletedObject{Key: obj.Key}
	}
	return &s3.DeleteObjectsOutput{Deleted: deleted}, nil
}

func (f *fakeS3) HeadBucket(_ context.Context, _ *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return &s3.HeadBucketOutput{}, f.headErr
}

// fakePresigner is a hand-rolled presignAPI.
type fakePresigner struct {
	gotExpiry time.Duration
	url       string
	err       error
}

func (f *fakePresigner) PresignGetObject(_ context.Context, _ *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4signer.PresignedHTTPRequest, error) {
	var opts s3.PresignOptions
	for _, fn := range optFns {
		fn(&opts)
	}
	f.gotExpiry = opts.Expires
	if f.err != nil {
		return nil, f.err
	}
	url := f.url
	if url == "" {
		url = "https://example.com/presigned"
	}
	return &v4signer.PresignedHTTPRequest{URL: url}, nil
}

func newTestStorage(client *fakeS3, presigner *fakePresigner, cfg Config) *fileStorage {
	if cfg.BucketName == "" {
		cfg = Config{BucketName: "bucket", AccountID: "acct", AccessKeyID: "k", SecretAccessKey: "s"}
	}
	return newFileStorage(client, presigner, ProviderR2, cfg)
}

func TestUpload(t *testing.T) {
	client := &fakeS3{}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	result, err := fs.Upload(context.Background(), "avatars/42.png", strings.NewReader("hello"), "image/png", 5)
	if err != nil {
		t.Fatalf("Upload() error = %v, want nil", err)
	}
	if result.Key != "avatars/42.png" || result.Size != 5 || result.ContentType != "image/png" {
		t.Errorf("Upload() = %+v, want key/size/contentType echoed back", result)
	}
	if result.ETag != "deadbeef" {
		t.Errorf("Upload() ETag = %q, want quotes stripped", result.ETag)
	}
	if result.URL != "" {
		t.Errorf("Upload() URL = %q, want empty when UsePublicURL is off", result.URL)
	}
	if aws.ToString(client.putIn.Bucket) != "bucket" || aws.ToString(client.putIn.Key) != "avatars/42.png" {
		t.Errorf("PutObjectInput = %+v, want it addressed at the configured bucket and given key", client.putIn)
	}
}

func TestUploadPublicURL(t *testing.T) {
	client := &fakeS3{}
	cfg := Config{BucketName: "bucket", AccountID: "acct", AccessKeyID: "k", SecretAccessKey: "s", UsePublicURL: true}
	fs := newTestStorage(client, &fakePresigner{}, cfg)

	result, err := fs.Upload(context.Background(), "a/b.png", strings.NewReader("x"), "image/png", 1)
	if err != nil {
		t.Fatalf("Upload() error = %v, want nil", err)
	}
	want := "https://pub-acct.r2.dev/a/b.png"
	if result.URL != want {
		t.Errorf("Upload() URL = %q, want %q", result.URL, want)
	}
}

func TestUploadRejectsInvalidKey(t *testing.T) {
	client := &fakeS3{}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	if _, err := fs.Upload(context.Background(), "", strings.NewReader("x"), "text/plain", 1); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Upload(empty key) error = %v, want ErrInvalidKey", err)
	}
	if client.putIn != nil {
		t.Error("Upload(empty key) called PutObject, want it rejected before the request was built")
	}
}

func TestUploadWrapsClientError(t *testing.T) {
	client := &fakeS3{putErr: errors.New("network exploded")}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	_, err := fs.Upload(context.Background(), "a.png", strings.NewReader("x"), "image/png", 1)
	if err == nil || !strings.Contains(err.Error(), "network exploded") {
		t.Errorf("Upload() error = %v, want it to wrap the client error", err)
	}
}

func TestDownload(t *testing.T) {
	body := io.NopCloser(strings.NewReader("content"))
	client := &fakeS3{getOut: &s3.GetObjectOutput{Body: body}}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	rc, err := fs.Download(context.Background(), "a/b.txt")
	if err != nil {
		t.Fatalf("Download() error = %v, want nil", err)
	}
	if rc != body {
		t.Error("Download() did not return the client's response body")
	}
}

func TestGetFileForDownloadDefaultsContentType(t *testing.T) {
	client := &fakeS3{getOut: &s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("x")),
		ContentLength: aws.Int64(1),
	}}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	dl, err := fs.GetFileForDownload(context.Background(), "a.bin")
	if err != nil {
		t.Fatalf("GetFileForDownload() error = %v, want nil", err)
	}
	if dl.ContentType != "application/octet-stream" {
		t.Errorf("ContentType = %q, want the default when the provider sent none", dl.ContentType)
	}
	if dl.Size != 1 {
		t.Errorf("Size = %d, want 1", dl.Size)
	}
}

func TestDelete(t *testing.T) {
	client := &fakeS3{}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	if err := fs.Delete(context.Background(), "a.txt"); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if aws.ToString(client.deleteIn.Key) != "a.txt" {
		t.Errorf("DeleteObjectInput.Key = %q, want %q", aws.ToString(client.deleteIn.Key), "a.txt")
	}
}

func TestBulkDeleteEmpty(t *testing.T) {
	client := &fakeS3{}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	result, err := fs.BulkDelete(context.Background(), nil)
	if err != nil || !result.OK() || len(result.Deleted) != 0 {
		t.Errorf("BulkDelete(nil) = %+v, %v, want an empty, OK result and no client call", result, err)
	}
	if len(client.deleteObjectsIn) != 0 {
		t.Error("BulkDelete(nil) called DeleteObjects, want no client call for an empty batch")
	}
}

func TestBulkDeleteRejectsInvalidKeyBeforeAnyCall(t *testing.T) {
	client := &fakeS3{}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	_, err := fs.BulkDelete(context.Background(), []string{"good.txt", ""})
	if !errors.Is(err, ErrInvalidKey) {
		t.Errorf("BulkDelete() error = %v, want ErrInvalidKey", err)
	}
	if len(client.deleteObjectsIn) != 0 {
		t.Error("BulkDelete() called DeleteObjects before validating every key")
	}
}

func TestBulkDeleteSplitsIntoBatches(t *testing.T) {
	client := &fakeS3{}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	keys := make([]string, MaxBatchSize+1)
	for i := range keys {
		keys[i] = fmt.Sprintf("k%04d", i)
	}

	result, err := fs.BulkDelete(context.Background(), keys)
	if err != nil {
		t.Fatalf("BulkDelete() error = %v, want nil", err)
	}
	if len(client.deleteObjectsIn) != 2 {
		t.Fatalf("DeleteObjects was called %d times, want 2 batches for %d keys", len(client.deleteObjectsIn), len(keys))
	}
	if len(client.deleteObjectsIn[0].Delete.Objects) != MaxBatchSize {
		t.Errorf("first batch had %d objects, want %d", len(client.deleteObjectsIn[0].Delete.Objects), MaxBatchSize)
	}
	if len(client.deleteObjectsIn[1].Delete.Objects) != 1 {
		t.Errorf("second batch had %d objects, want 1", len(client.deleteObjectsIn[1].Delete.Objects))
	}
	if len(result.Deleted) != len(keys) {
		t.Errorf("Deleted has %d keys, want all %d", len(result.Deleted), len(keys))
	}
}

func TestBulkDeleteReportsPartialFailure(t *testing.T) {
	client := &fakeS3{
		deleteObjectsOut: []*s3.DeleteObjectsOutput{{
			Deleted: []types.DeletedObject{{Key: aws.String("a")}},
			Errors:  []types.Error{{Key: aws.String("b"), Message: aws.String("access denied")}},
		}},
	}
	fs := newTestStorage(client, &fakePresigner{}, Config{})

	result, err := fs.BulkDelete(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("BulkDelete() error = %v, want nil (a partial failure is not a call error)", err)
	}
	if result.OK() {
		t.Error("DeleteResult.OK() = true, want false when a key failed")
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "a" {
		t.Errorf("Deleted = %v, want [a]", result.Deleted)
	}
	if len(result.Failed) != 1 || result.Failed[0].Key != "b" || result.Failed[0].Reason != "access denied" {
		t.Errorf("Failed = %+v, want one failure naming key b and its reason", result.Failed)
	}
}

func TestGetPresignedURLDefaultsExpiry(t *testing.T) {
	presigner := &fakePresigner{}
	fs := newTestStorage(&fakeS3{}, presigner, Config{})

	if _, err := fs.GetPresignedURL(context.Background(), "a.txt", 0); err != nil {
		t.Fatalf("GetPresignedURL() error = %v, want nil", err)
	}
	if presigner.gotExpiry != DefaultPresignExpiry {
		t.Errorf("expiry sent to the presigner = %v, want %v", presigner.gotExpiry, DefaultPresignExpiry)
	}
}

func TestGetPresignedURLRejectsExpiryTooLong(t *testing.T) {
	fs := newTestStorage(&fakeS3{}, &fakePresigner{}, Config{})

	_, err := fs.GetPresignedURL(context.Background(), "a.txt", MaxPresignExpiry+time.Hour)
	if err == nil {
		t.Fatal("GetPresignedURL() error = nil, want a rejection past MaxPresignExpiry")
	}
}

func TestGetPublicURLEscapesReservedCharacters(t *testing.T) {
	fs := newTestStorage(&fakeS3{}, &fakePresigner{}, Config{})

	url, err := fs.GetPublicURL("a b/c#d.png")
	if err != nil {
		t.Fatalf("GetPublicURL() error = %v, want nil", err)
	}
	want := "https://pub-acct.r2.dev/a%20b/c%23d.png"
	if url != want {
		t.Errorf("GetPublicURL() = %q, want %q", url, want)
	}
}

func TestGetPublicURLPrefersCustomDomain(t *testing.T) {
	cfg := Config{BucketName: "b", AccountID: "acct", AccessKeyID: "k", SecretAccessKey: "s", PublicDomain: "cdn.example.com"}
	fs := newTestStorage(&fakeS3{}, &fakePresigner{}, cfg)

	url, err := fs.GetPublicURL("a.png")
	if err != nil {
		t.Fatalf("GetPublicURL() error = %v, want nil", err)
	}
	if want := "https://cdn.example.com/a.png"; url != want {
		t.Errorf("GetPublicURL() = %q, want %q", url, want)
	}
}

func TestGetPublicURLUnsupportedProvider(t *testing.T) {
	fs := newFileStorage(&fakeS3{}, &fakePresigner{}, Provider("gcs"), Config{
		BucketName: "b", AccountID: "a", AccessKeyID: "k", SecretAccessKey: "s",
	})

	_, err := fs.GetPublicURL("a.png")
	if !errors.Is(err, ErrUnsupportedProvider) {
		t.Errorf("GetPublicURL() error = %v, want ErrUnsupportedProvider", err)
	}
}

func TestIsNotFound(t *testing.T) {
	if IsNotFound(nil) {
		t.Error("IsNotFound(nil) = true, want false")
	}
	if IsNotFound(errors.New("boom")) {
		t.Error("IsNotFound(plain error) = true, want false")
	}
	if !IsNotFound(&types.NoSuchKey{}) {
		t.Error("IsNotFound(*types.NoSuchKey) = false, want true")
	}
	if !IsNotFound(&types.NotFound{}) {
		t.Error("IsNotFound(*types.NotFound) = false, want true")
	}
	wrapped := fmt.Errorf("get object: %w", &types.NoSuchKey{})
	if !IsNotFound(wrapped) {
		t.Error("IsNotFound should see through wrapping")
	}
}
