package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// newClient builds the client for provider from cfg. It is the one place a
// new [Provider] needs to be wired in.
func newClient(ctx context.Context, provider Provider, cfg Config) (*s3.Client, error) {
	switch provider {
	case ProviderR2:
		return newR2Client(ctx, cfg)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedProvider, provider)
	}
}

// newR2Client builds a client for Cloudflare R2, an S3-compatible store
// reached through a per-account endpoint rather than AWS's regional ones.
func newR2Client(ctx context.Context, cfg Config) (*s3.Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		awsconfig.WithRegion("auto"),
		// R2 does not support S3's request/response checksum features; leaving
		// them at their default trips checksum errors R2 has no way to satisfy.
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationUnset),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationUnset),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: load AWS config: %w", err)
	}

	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID))
		o.UsePathStyle = true
	}), nil
}
