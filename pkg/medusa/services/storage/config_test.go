package storage

import (
	"errors"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{
		BucketName:      "assets",
		AccountID:       "account",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
	}

	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for a fully populated config", err)
	}

	tests := map[string]Config{
		"missing bucket":     {AccountID: "a", AccessKeyID: "k", SecretAccessKey: "s"},
		"missing account":    {BucketName: "b", AccessKeyID: "k", SecretAccessKey: "s"},
		"missing access key": {BucketName: "b", AccountID: "a", SecretAccessKey: "s"},
		"missing secret":     {BucketName: "b", AccountID: "a", AccessKeyID: "k"},
		"all missing":        {},
	}

	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error for %+v", cfg)
			}
			if !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("Validate() = %v, want it to wrap ErrInvalidConfig", err)
			}
		})
	}
}

func TestProviderIsValid(t *testing.T) {
	if !ProviderR2.IsValid() {
		t.Error("ProviderR2.IsValid() = false, want true")
	}
	if Provider("gcs").IsValid() {
		t.Error(`Provider("gcs").IsValid() = true, want false`)
	}
	if Provider("").IsValid() {
		t.Error(`Provider("").IsValid() = true, want false`)
	}
}
