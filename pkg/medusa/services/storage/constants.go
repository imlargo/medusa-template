package storage

import "time"

// MaxBatchSize is the most objects a single DeleteObjects call may name.
// This is S3's own limit for bulk delete, not this package's choice;
// [FileStorage.BulkDelete] splits a larger request into batches of this size.
const MaxBatchSize = 1000

// connectTimeout bounds the HeadBucket call [NewFileStorage] makes to confirm
// the bucket is reachable before returning.
const connectTimeout = 10 * time.Second
