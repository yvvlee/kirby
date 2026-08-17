package object

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/safeint"
)

const (
	declaredSizeMetadata        = "kirby-declared-size"
	declaredContentTypeMetadata = "kirby-declared-content-type"
	declaredSizeHeader          = "X-Amz-Meta-Kirby-Declared-Size"
	declaredContentTypeHeader   = "X-Amz-Meta-Kirby-Declared-Content-Type"
)

var safeProviderCode = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

type s3PresignAPI interface {
	PresignedPostPolicy(context.Context, *minio.PostPolicy) (*url.URL, map[string]string, error)
}

type s3InternalAPI interface {
	StatObject(context.Context, string, string, minio.StatObjectOptions) (minio.ObjectInfo, error)
	OpenObject(context.Context, string, string, string) (io.ReadCloser, error)
	PutObjectIfAbsent(context.Context, string, string, io.Reader, int64, string, map[string]string) (minio.UploadInfo, error)
	RemoveObject(context.Context, string, string, minio.RemoveObjectOptions) error
	BucketExists(context.Context, string) (bool, error)
}

type minioInternalClient struct {
	*minio.Client
}

func (client *minioInternalClient) OpenObject(ctx context.Context, bucket, key, etag string) (io.ReadCloser, error) {
	options := minio.GetObjectOptions{}
	if err := options.SetMatchETag(etag); err != nil {
		return nil, err
	}
	return client.GetObject(ctx, bucket, key, options)
}

func (client *minioInternalClient) PutObjectIfAbsent(
	ctx context.Context,
	bucket string,
	key string,
	source io.Reader,
	size int64,
	contentType string,
	metadata map[string]string,
) (minio.UploadInfo, error) {
	options := minio.PutObjectOptions{
		ContentType:      contentType,
		UserMetadata:     metadata,
		DisableMultipart: true,
	}
	options.SetMatchETagExcept("*")
	return client.PutObject(ctx, bucket, key, source, size, options)
}

// S3Storage implements ObjectStorage against an S3-compatible provider.
type S3Storage struct {
	internalClient s3InternalAPI
	presignClient  s3PresignAPI
	bucket         string
	publicBaseURL  url.URL
	now            func() time.Time
}

// NewS3 constructs an S3-compatible adapter without creating a bucket.
func NewS3(cfg config.S3Config) (*S3Storage, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	sharedCredentials := credentials.NewStaticV4(cfg.AccessKey.Value(), cfg.SecretKey.Value(), "")
	internalClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  sharedCredentials,
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 client: invalid configuration")
	}
	presignClient, err := minio.New(cfg.PresignEndpoint, &minio.Options{
		Creds:  sharedCredentials,
		Secure: *cfg.PresignUseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 presign client: invalid configuration")
	}
	publicBaseURL, err := url.Parse(cfg.PublicBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse S3 public base URL: invalid configuration")
	}
	return newS3(&minioInternalClient{Client: internalClient}, presignClient, cfg.Bucket, publicBaseURL, time.Now), nil
}

func newS3(internalClient s3InternalAPI, presignClient s3PresignAPI, bucket string, publicBaseURL *url.URL, now func() time.Time) *S3Storage {
	return &S3Storage{
		internalClient: internalClient,
		presignClient:  presignClient,
		bucket:         bucket,
		publicBaseURL:  *publicBaseURL,
		now:            now,
	}
}

func (s *S3Storage) bucketExists(ctx context.Context) (bool, error) {
	exists, err := s.internalClient.BucketExists(ctx, s.bucket)
	if err != nil {
		return false, safeS3Error("check S3 bucket", err)
	}
	return exists, nil
}

// PresignUpload creates a POST policy whose content-length-range is enforced
// by S3 while it receives the request body. Management credentials never
// cross the object storage boundary.
func (s *S3Storage) PresignUpload(ctx context.Context, input PresignUploadInput) (*UploadTicket, error) {
	contentType, err := validatePresignInput(input)
	if err != nil {
		return nil, err
	}
	scope, err := ParseObjectKey(input.Key)
	if err != nil || scope.Temporary {
		return nil, fmt.Errorf("%w: S3 presign requires a final scoped key", ErrInvalidInput)
	}
	uploadKey, err := BuildUploadObjectKey(scope.EnvironmentID, scope.ProjectID, scope.ObjectID, scope.Extension)
	if err != nil {
		return nil, err
	}
	expiresAt := s.now().UTC().Add(input.ExpiresIn)
	policy := minio.NewPostPolicy()
	if err := policy.SetBucket(s.bucket); err != nil {
		return nil, fmt.Errorf("%w: build S3 upload policy", ErrInvalidInput)
	}
	if err := policy.SetKey(uploadKey); err != nil {
		return nil, fmt.Errorf("%w: build S3 upload policy", ErrInvalidInput)
	}
	if err := policy.SetExpires(expiresAt); err != nil {
		return nil, fmt.Errorf("%w: build S3 upload policy", ErrInvalidInput)
	}
	if err := policy.SetContentType(contentType); err != nil {
		return nil, fmt.Errorf("%w: build S3 upload policy", ErrInvalidInput)
	}
	if err := policy.SetContentLengthRange(1, int64(MaxUploadSize)); err != nil {
		return nil, fmt.Errorf("%w: build S3 upload policy", ErrInvalidInput)
	}
	if err := policy.SetUserMetadata(declaredSizeMetadata, strconv.FormatUint(input.Size, 10)); err != nil {
		return nil, fmt.Errorf("%w: build S3 upload policy", ErrInvalidInput)
	}
	if err := policy.SetUserMetadata(declaredContentTypeMetadata, contentType); err != nil {
		return nil, fmt.Errorf("%w: build S3 upload policy", ErrInvalidInput)
	}
	uploadURL, formFields, err := s.presignClient.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return nil, safeS3Error("presign S3 upload", err)
	}
	return &UploadTicket{
		Key:        uploadKey,
		URL:        uploadURL.String(),
		Method:     UploadMethodPost,
		Headers:    map[string]string{},
		FormFields: cloneStringMap(formFields),
		ExpiresAt:  expiresAt,
	}, nil
}

// CompleteUpload verifies a private temporary object, streams that exact ETag
// into a conditionally created public object, and deletes the temporary source.
// Reusing an upload ticket can therefore never change a published URL.
func (s *S3Storage) CompleteUpload(ctx context.Context, key string) (*Metadata, error) {
	scope, err := ParseObjectKey(key)
	if err != nil {
		return nil, err
	}
	if !scope.Temporary {
		return nil, fmt.Errorf("%w: only temporary S3 upload keys can be completed", ErrInvalidInput)
	}
	metadata, err := s.ReadMetadata(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := validateCompletedMetadata(metadata); err != nil {
		return nil, err
	}
	finalKey, err := BuildObjectKey(scope.EnvironmentID, scope.ProjectID, scope.ObjectID, scope.Extension)
	if err != nil {
		return nil, fmt.Errorf("generate final S3 object key: %w", ErrStorageUnavailable)
	}
	source, err := s.internalClient.OpenObject(ctx, s.bucket, key, metadata.ETag)
	if err != nil {
		return nil, safeS3Error("open validated S3 upload", err)
	}
	defer source.Close()
	publishSize, err := safeint.Int64FromUint64(metadata.Size)
	if err != nil {
		return nil, fmt.Errorf("%w: S3 object size is invalid", ErrObjectIntegrity)
	}
	publishInfo, err := s.internalClient.PutObjectIfAbsent(
		ctx,
		s.bucket,
		finalKey,
		source,
		publishSize,
		metadata.ContentType,
		map[string]string{
			declaredSizeMetadata:        strconv.FormatUint(metadata.DeclaredSize, 10),
			declaredContentTypeMetadata: metadata.DeclaredContentType,
		},
	)
	if err != nil {
		return nil, safeS3Error("publish S3 object", err)
	}
	// Publishing is the commit point. A failed temporary-object cleanup must
	// not delete or hide the immutable final object. The required uploads/
	// lifecycle rule removes this bounded residue asynchronously.
	_ = s.internalClient.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	published := *metadata
	published.Key = finalKey
	published.URL = s.publicURL(finalKey)
	published.ETag = publishInfo.ETag
	if !publishInfo.LastModified.IsZero() {
		published.LastModified = publishInfo.LastModified.UTC()
	}
	return &published, nil
}

// ReadMetadata calls HEAD/Stat on the object provider.
func (s *S3Storage) ReadMetadata(ctx context.Context, key string) (*Metadata, error) {
	scope, err := ParseObjectKey(key)
	if err != nil {
		return nil, err
	}
	info, err := s.internalClient.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, safeS3Error("read S3 object metadata", err)
	}
	if info.Size < 0 {
		return nil, fmt.Errorf("%w: S3 object size is invalid", ErrObjectIntegrity)
	}
	if strings.TrimSpace(info.ETag) == "" {
		return nil, fmt.Errorf("%w: S3 object ETag is missing", ErrObjectIntegrity)
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
	objectURL := ""
	if !scope.Temporary {
		objectURL = s.publicURL(key)
	}
	objectSize, err := safeint.Uint64FromInt64(info.Size)
	if err != nil {
		return nil, fmt.Errorf("%w: S3 object size is invalid", ErrObjectIntegrity)
	}
	return &Metadata{
		Key:                 key,
		URL:                 objectURL,
		ETag:                info.ETag,
		ContentType:         info.ContentType,
		Size:                objectSize,
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
	if err := s.internalClient.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return safeS3Error("remove incomplete S3 object", err)
	}
	return nil
}

// Close is present for uniform provider shutdown. minio.Client owns no
// closeable background connection pool.
func (*S3Storage) Close() error { return nil }

func (s *S3Storage) publicURL(key string) string {
	publicURL := s.publicBaseURL
	publicURL.Path = path.Join(publicURL.Path, key)
	return publicURL.String()
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

func cloneStringMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
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
	if response.Code == "PreconditionFailed" || response.Code == "ConditionNotMet" || response.StatusCode == http.StatusPreconditionFailed {
		return fmt.Errorf("%s: %w", operation, ErrObjectConflict)
	}
	if safeProviderCode.MatchString(response.Code) {
		return fmt.Errorf("%s: %w (provider code %s)", operation, ErrStorageUnavailable, response.Code)
	}
	return fmt.Errorf("%s: %w", operation, ErrStorageUnavailable)
}
