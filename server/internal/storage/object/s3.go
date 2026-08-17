package object

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/yvvlee/kirby/server/internal/config"
)

const (
	declaredSizeHeader        = "X-Amz-Meta-Kirby-Declared-Size"
	declaredContentTypeHeader = "X-Amz-Meta-Kirby-Declared-Content-Type"
)

var safeProviderCode = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

type s3API interface {
	PresignHeader(context.Context, string, string, string, time.Duration, url.Values, http.Header) (*url.URL, error)
	StatObject(context.Context, string, string, minio.StatObjectOptions) (minio.ObjectInfo, error)
	RemoveObject(context.Context, string, string, minio.RemoveObjectOptions) error
	BucketExists(context.Context, string) (bool, error)
}

// S3Storage implements ObjectStorage against an S3-compatible provider.
type S3Storage struct {
	client   s3API
	bucket   string
	endpoint string
	scheme   string
	now      func() time.Time
}

// NewS3 constructs an S3-compatible adapter without creating a bucket.
func NewS3(cfg config.S3Config) (*S3Storage, error) {
	endpoint, err := validateS3Endpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}
	if cfg.AccessKey.Empty() || cfg.SecretKey.Empty() {
		return nil, fmt.Errorf("S3 credentials are required")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey.Value(), cfg.SecretKey.Value(), ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 client: invalid configuration")
	}
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	return newS3(client, cfg.Bucket, endpoint, scheme, time.Now), nil
}

func newS3(client s3API, bucket, endpoint, scheme string, now func() time.Time) *S3Storage {
	return &S3Storage{client: client, bucket: bucket, endpoint: endpoint, scheme: scheme, now: now}
}

func (s *S3Storage) bucketExists(ctx context.Context) (bool, error) {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return false, safeS3Error("check S3 bucket", err)
	}
	return exists, nil
}

// PresignUpload signs only storage-specific headers. Management credentials
// never cross the object storage boundary.
func (s *S3Storage) PresignUpload(ctx context.Context, input PresignUploadInput) (*UploadTicket, error) {
	contentType, err := validatePresignInput(input)
	if err != nil {
		return nil, err
	}
	headers := http.Header{}
	headers.Set("Content-Type", contentType)
	headers.Set(declaredSizeHeader, strconv.FormatUint(input.Size, 10))
	headers.Set(declaredContentTypeHeader, contentType)
	uploadURL, err := s.client.PresignHeader(
		ctx,
		http.MethodPut,
		s.bucket,
		input.Key,
		input.ExpiresIn,
		nil,
		headers,
	)
	if err != nil {
		return nil, safeS3Error("presign S3 upload", err)
	}
	expiresAt := s.now().UTC().Add(input.ExpiresIn)
	return &UploadTicket{
		Key:       input.Key,
		URL:       uploadURL.String(),
		Headers:   firstHeaderValues(headers),
		ExpiresAt: expiresAt,
	}, nil
}

// CompleteUpload verifies actual provider metadata against signed upload
// declarations stored with the object.
func (s *S3Storage) CompleteUpload(ctx context.Context, key string) (*Metadata, error) {
	metadata, err := s.ReadMetadata(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := validateCompletedMetadata(metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

// ReadMetadata calls HEAD/Stat on the object provider.
func (s *S3Storage) ReadMetadata(ctx context.Context, key string) (*Metadata, error) {
	if _, err := ParseObjectKey(key); err != nil {
		return nil, err
	}
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, safeS3Error("read S3 object metadata", err)
	}
	if info.Size < 0 {
		return nil, fmt.Errorf("%w: S3 object size is invalid", ErrObjectIntegrity)
	}
	declaredSizeValue := metadataValue(info.UserMetadata, declaredSizeHeader)
	declaredSize, err := strconv.ParseUint(declaredSizeValue, 10, 64)
	if err != nil || declaredSize == 0 {
		return nil, fmt.Errorf("%w: S3 object size declaration is invalid", ErrObjectIntegrity)
	}
	declaredContentType := metadataValue(info.UserMetadata, declaredContentTypeHeader)
	if declaredContentType == "" {
		return nil, fmt.Errorf("%w: S3 content type declaration is missing", ErrObjectIntegrity)
	}
	return &Metadata{
		Key:                 key,
		URL:                 s.publicURL(key),
		ContentType:         info.ContentType,
		Size:                uint64(info.Size),
		DeclaredContentType: declaredContentType,
		DeclaredSize:        declaredSize,
		LastModified:        info.LastModified.UTC(),
	}, nil
}

// DeleteIncomplete deletes an object that failed final validation.
func (s *S3Storage) DeleteIncomplete(ctx context.Context, key string) error {
	if _, err := ParseObjectKey(key); err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return safeS3Error("remove incomplete S3 object", err)
	}
	return nil
}

// Close is present for uniform provider shutdown. minio.Client owns no
// closeable background connection pool.
func (*S3Storage) Close() error { return nil }

func (s *S3Storage) publicURL(key string) string {
	return (&url.URL{
		Scheme: s.scheme,
		Host:   s.endpoint,
		Path:   "/" + s.bucket + "/" + key,
	}).String()
}

func validateS3Endpoint(endpoint string) (string, error) {
	if strings.TrimSpace(endpoint) != endpoint || endpoint == "" || strings.Contains(endpoint, "://") || strings.ContainsAny(endpoint, "/?#@") {
		return "", fmt.Errorf("S3 endpoint must be a host with optional port and no scheme or path")
	}
	parsed, err := url.Parse("http://" + endpoint)
	if err != nil || parsed.Host != endpoint || parsed.Hostname() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", fmt.Errorf("S3 endpoint must be a host with optional port and no scheme or path")
	}
	return endpoint, nil
}

func metadataValue(metadata map[string]string, name string) string {
	wanted := strings.ToLower(strings.TrimPrefix(name, "X-Amz-Meta-"))
	for key, value := range metadata {
		normalized := strings.ToLower(strings.TrimPrefix(strings.ToLower(key), "x-amz-meta-"))
		if normalized == wanted {
			return value
		}
	}
	return ""
}

func firstHeaderValues(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

func safeS3Error(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	response := minio.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" || response.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s: %w", operation, ErrObjectNotFound)
	}
	if safeProviderCode.MatchString(response.Code) {
		return fmt.Errorf("%s: %w (provider code %s)", operation, ErrStorageUnavailable, response.Code)
	}
	return fmt.Errorf("%s: %w", operation, ErrStorageUnavailable)
}
