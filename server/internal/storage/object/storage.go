// Package object provides the object storage boundary used by asset uploads.
package object

import (
	"context"
	"errors"
	"fmt"
	"math"
	"mime"
	"strings"
	"time"

	"github.com/yvvlee/kirby/server/internal/config"
)

const (
	// LocalUploadPath is registered by the HTTP server for direct local uploads.
	LocalUploadPath = "/api/assets/upload"
	// LocalObjectPathPrefix is registered by the HTTP server for local assets.
	LocalObjectPathPrefix = "/api/assets/objects/"
	// DefaultUploadTTL limits how long a browser upload ticket remains valid.
	DefaultUploadTTL = 15 * time.Minute
)

var (
	ErrInvalidInput       = errors.New("invalid object storage input")
	ErrObjectNotFound     = errors.New("object not found")
	ErrObjectIntegrity    = errors.New("object metadata does not match upload declaration")
	ErrStorageUnavailable = errors.New("object storage unavailable")
)

// PresignUploadInput describes one direct browser upload.
type PresignUploadInput struct {
	Key         string
	ContentType string
	Size        uint64
	ExpiresIn   time.Duration
}

// UploadTicket contains only the data the browser sends to object storage.
// It must never contain a management JWT, Cookie, or project API key.
type UploadTicket struct {
	Key       string
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

// Metadata is read from storage after the browser finishes uploading.
type Metadata struct {
	Key                 string
	URL                 string
	ContentType         string
	Size                uint64
	DeclaredContentType string
	DeclaredSize        uint64
	LastModified        time.Time
}

// ObjectStorage is the provider-neutral upload contract.
type ObjectStorage interface {
	PresignUpload(context.Context, PresignUploadInput) (*UploadTicket, error)
	CompleteUpload(context.Context, string) (*Metadata, error)
	ReadMetadata(context.Context, string) (*Metadata, error)
	DeleteIncomplete(context.Context, string) error
	Close() error
}

// Open creates the configured adapter. It repeats the multi-instance guard so
// callers cannot bypass it by constructing an unchecked Config value.
func Open(ctx context.Context, mode config.DeploymentMode, cfg config.ObjectStorageConfig) (ObjectStorage, error) {
	if mode == config.ModeMulti && cfg.Driver == "local" {
		return nil, fmt.Errorf("mode=multi cannot use local object storage")
	}
	switch cfg.Driver {
	case "local":
		return NewLocal(cfg.Local.Directory)
	case "s3":
		storage, err := NewS3(cfg.S3)
		if err != nil {
			return nil, err
		}
		exists, err := storage.bucketExists(ctx)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("configured S3 bucket does not exist")
		}
		return storage, nil
	default:
		return nil, fmt.Errorf("unsupported object storage driver %q", cfg.Driver)
	}
}

func validatePresignInput(input PresignUploadInput) (string, error) {
	if _, err := ParseObjectKey(input.Key); err != nil {
		return "", err
	}
	if input.Size == 0 || input.Size > math.MaxInt64-1 {
		return "", fmt.Errorf("%w: size must be between one and the supported maximum", ErrInvalidInput)
	}
	contentType, err := normalizeContentType(input.ContentType)
	if err != nil {
		return "", err
	}
	if input.ExpiresIn <= 0 || input.ExpiresIn > 24*time.Hour {
		return "", fmt.Errorf("%w: upload expiry must be between zero and 24 hours", ErrInvalidInput)
	}
	return contentType, nil
}

func validateCompletedMetadata(metadata *Metadata) error {
	if metadata == nil {
		return fmt.Errorf("%w: metadata is missing", ErrObjectIntegrity)
	}
	if metadata.Size == 0 || metadata.DeclaredSize == 0 || metadata.Size != metadata.DeclaredSize {
		return fmt.Errorf("%w: object size differs from its signed declaration", ErrObjectIntegrity)
	}
	actualType, err := normalizeContentType(metadata.ContentType)
	if err != nil {
		return fmt.Errorf("%w: object content type is invalid", ErrObjectIntegrity)
	}
	declaredType, err := normalizeContentType(metadata.DeclaredContentType)
	if err != nil || actualType != declaredType {
		return fmt.Errorf("%w: object content type differs from its signed declaration", ErrObjectIntegrity)
	}
	metadata.ContentType = actualType
	metadata.DeclaredContentType = declaredType
	return nil
}

func normalizeContentType(value string) (string, error) {
	if strings.TrimSpace(value) != value || value == "" || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%w: content type is invalid", ErrInvalidInput)
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || !strings.Contains(mediaType, "/") {
		return "", fmt.Errorf("%w: content type is invalid", ErrInvalidInput)
	}
	return strings.ToLower(mediaType), nil
}
