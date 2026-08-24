package storage

import (
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// ErrUnsupportedProvider is returned when a [Provider] has no registered
// client constructor, or no public-URL scheme. It is a configuration error,
// reported at construction rather than on the first request.
var ErrUnsupportedProvider = errors.New("storage: unsupported provider")

// ErrInvalidKey is the class of every key [ValidateKey] rejects.
var ErrInvalidKey = errors.New("storage: invalid key")

// ErrInvalidConfig is the class of every problem [Config.Validate] finds.
var ErrInvalidConfig = errors.New("storage: invalid config")

// IsNotFound reports whether err means the requested object, or bucket, does
// not exist.
//
// It checks both the typed errors the AWS SDK decodes a 404 into and, as a
// fallback, a bare HTTP 404 — because an S3-compatible provider that is not
// S3 itself does not always serialize an error body the SDK recognizes as one
// of its own types, but it still sends the status code.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := errors.AsType[*types.NoSuchKey](err); ok {
		return true
	}
	if _, ok := errors.AsType[*types.NotFound](err); ok {
		return true
	}
	if respErr, ok := errors.AsType[*smithyhttp.ResponseError](err); ok {
		return respErr.HTTPStatusCode() == http.StatusNotFound
	}

	return false
}
