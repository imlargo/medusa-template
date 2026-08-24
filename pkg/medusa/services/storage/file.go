package storage

import "io"

// File is what an application-level file service hands down when it has a
// reader and the metadata around it, but has not yet decided the object key.
// It is a convenience DTO for that layer to pass around — [FileStorage.Upload]
// itself takes its fields directly and does not accept a File, since Upload
// needs a key the caller has already chosen and File does not carry one.
type File struct {
	Reader      io.Reader
	Filename    string
	Size        int64
	ContentType string
}

// FileDownload is a downloaded object together with the metadata the
// provider reported for it. Content must be closed by the caller.
type FileDownload struct {
	Content     io.ReadCloser
	ContentType string
	Size        int64
}

// FileResult describes an object after it has been written.
type FileResult struct {
	Key         string
	Size        int64
	ContentType string
	ETag        string
	URL         string // empty unless Config.UsePublicURL
}

// DeleteResult reports the outcome of a BulkDelete, key by key.
//
// A single combined error cannot say which of a thousand keys failed and
// which are now gone. A batch where three keys are denied by a bucket policy
// is not one error; it is 997 successes and three named failures, and folding
// that into a string would throw away exactly the information a caller needs
// to retry only what failed.
type DeleteResult struct {
	Deleted []string
	Failed  []DeleteFailure
}

// OK reports whether every key was deleted.
func (r DeleteResult) OK() bool { return len(r.Failed) == 0 }

// DeleteFailure is one key BulkDelete could not remove, and why.
type DeleteFailure struct {
	Key    string
	Reason string
}
