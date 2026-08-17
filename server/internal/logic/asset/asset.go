// Package asset validates asset ownership and upload policy before touching
// object storage.
package asset

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/yvvlee/kirby/server/internal/storage/object"
)

const (
	PermissionAssetWrite = "asset:write"
	DefaultMaxSize       = object.MaxUploadSize
)

var (
	ErrInvalidInput    = errors.New("invalid asset input")
	ErrForbidden       = errors.New("asset operation is forbidden")
	ErrProjectNotFound = errors.New("project not found in environment")
	ErrAssetNotFound   = errors.New("asset not found")
	ErrAssetConflict   = errors.New("asset already published")
	ErrAssetIntegrity  = errors.New("asset failed integrity validation")
	ErrDependency      = errors.New("asset dependency failed")
)

// Authorizer checks the caller's role in one environment.
type Authorizer interface {
	HasEnvironmentPermission(context.Context, int64, string) (bool, error)
}

// ProjectScope checks that a project belongs to the selected environment.
type ProjectScope interface {
	ProjectExists(context.Context, int64, int64) (bool, error)
}

// Policy controls accepted extensions, MIME types, and size.
type Policy struct {
	MaxSize        uint64
	AllowedByExt   map[string][]string
	UploadLifetime time.Duration
}

// DefaultPolicy permits common image, media, document, and archive formats.
func DefaultPolicy() Policy {
	return Policy{
		MaxSize: DefaultMaxSize,
		AllowedByExt: map[string][]string{
			".png":  {"image/png"},
			".jpg":  {"image/jpeg"},
			".jpeg": {"image/jpeg"},
			".gif":  {"image/gif"},
			".webp": {"image/webp"},
			".avif": {"image/avif"},
			".mp4":  {"video/mp4"},
			".webm": {"video/webm"},
			".mov":  {"video/quicktime"},
			".mp3":  {"audio/mpeg"},
			".wav":  {"audio/wav", "audio/x-wav"},
			".ogg":  {"audio/ogg", "video/ogg"},
			".pdf":  {"application/pdf"},
			".txt":  {"text/plain"},
			".csv":  {"text/csv", "application/csv"},
			".json": {"application/json"},
			".yaml": {"application/yaml", "application/x-yaml", "text/yaml"},
			".yml":  {"application/yaml", "application/x-yaml", "text/yaml"},
			".zip":  {"application/zip", "application/x-zip-compressed"},
			".gz":   {"application/gzip", "application/x-gzip"},
			".tar":  {"application/x-tar"},
			".doc":  {"application/msword"},
			".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
			".xls":  {"application/vnd.ms-excel"},
			".xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
			".ppt":  {"application/vnd.ms-powerpoint"},
			".pptx": {"application/vnd.openxmlformats-officedocument.presentationml.presentation"},
		},
		UploadLifetime: object.DefaultUploadTTL,
	}
}

// PresignInput is the application-level upload request.
type PresignInput struct {
	EnvironmentID int64
	ProjectID     int64
	Filename      string
	ContentType   string
	Size          uint64
}

// Logic coordinates authorization, project ownership, policy, and storage.
type Logic struct {
	storage    object.ObjectStorage
	authorizer Authorizer
	projects   ProjectScope
	policy     Policy
	newID      func() (string, error)
}

// New constructs asset logic with the fixed public upload policy.
func New(storage object.ObjectStorage, authorizer Authorizer, projects ProjectScope) (*Logic, error) {
	return newLogic(storage, authorizer, projects, DefaultPolicy(), func() (string, error) {
		value, err := uuid.NewRandom()
		return value.String(), err
	})
}

func newLogic(storage object.ObjectStorage, authorizer Authorizer, projects ProjectScope, policy Policy, newID func() (string, error)) (*Logic, error) {
	if storage == nil || authorizer == nil || projects == nil || newID == nil {
		return nil, fmt.Errorf("asset dependencies must not be nil")
	}
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	return &Logic{storage: storage, authorizer: authorizer, projects: projects, policy: policy, newID: newID}, nil
}

// Presign authorizes the request and creates a scoped, unguessable key.
func (logic *Logic) Presign(ctx context.Context, input PresignInput) (*object.UploadTicket, error) {
	extension, contentType, err := logic.validateUpload(input.Filename, input.ContentType, input.Size)
	if err != nil {
		return nil, err
	}
	if input.EnvironmentID <= 0 || input.ProjectID <= 0 {
		return nil, fmt.Errorf("%w: environment and project IDs must be positive", ErrInvalidInput)
	}
	if err := logic.requireAccess(ctx, input.EnvironmentID, input.ProjectID); err != nil {
		return nil, err
	}
	objectID, err := logic.newID()
	if err != nil {
		return nil, fmt.Errorf("%w: generate object ID", ErrDependency)
	}
	key, err := object.BuildObjectKey(input.EnvironmentID, input.ProjectID, objectID, extension)
	if err != nil {
		return nil, fmt.Errorf("%w: generated object key is invalid", ErrDependency)
	}
	ticket, err := logic.storage.PresignUpload(ctx, object.PresignUploadInput{
		Key: key, ContentType: contentType, Size: input.Size, ExpiresIn: logic.policy.UploadLifetime,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: create upload ticket", ErrDependency)
	}
	return ticket, nil
}

// Complete validates key ownership before querying provider metadata.
func (logic *Logic) Complete(ctx context.Context, environmentID, projectID int64, key string) (*object.Metadata, error) {
	if environmentID <= 0 || projectID <= 0 {
		return nil, fmt.Errorf("%w: environment and project IDs must be positive", ErrInvalidInput)
	}
	scope, err := object.ParseObjectKey(key)
	if err != nil {
		return nil, fmt.Errorf("%w: object key is invalid", ErrInvalidInput)
	}
	if scope.EnvironmentID != environmentID || scope.ProjectID != projectID {
		return nil, fmt.Errorf("%w: object belongs to another environment or project", ErrForbidden)
	}
	if err := logic.requireAccess(ctx, environmentID, projectID); err != nil {
		return nil, err
	}
	metadata, err := logic.storage.CompleteUpload(ctx, key)
	if err != nil {
		switch {
		case errors.Is(err, object.ErrObjectNotFound):
			return nil, fmt.Errorf("%w", ErrAssetNotFound)
		case errors.Is(err, object.ErrObjectIntegrity):
			if deleteErr := logic.storage.DeleteIncomplete(ctx, key); deleteErr != nil {
				return nil, fmt.Errorf("%w: remove invalid object", ErrDependency)
			}
			return nil, fmt.Errorf("%w: stored metadata differs from upload declaration", ErrAssetIntegrity)
		case errors.Is(err, object.ErrObjectConflict):
			return nil, fmt.Errorf("%w", ErrAssetConflict)
		default:
			return nil, fmt.Errorf("%w: inspect uploaded object", ErrDependency)
		}
	}
	if metadata == nil {
		return nil, fmt.Errorf("%w: storage returned no published object", ErrDependency)
	}
	publishedScope, err := object.ParseObjectKey(metadata.Key)
	if err != nil || publishedScope.Temporary ||
		publishedScope.EnvironmentID != environmentID || publishedScope.ProjectID != projectID ||
		publishedScope.Extension != scope.Extension {
		return nil, fmt.Errorf("%w: storage returned an invalid published object scope", ErrAssetIntegrity)
	}
	if err := logic.validateCompleted(scope.Extension, metadata); err != nil {
		if deleteErr := logic.storage.DeleteIncomplete(ctx, metadata.Key); deleteErr != nil {
			return nil, fmt.Errorf("%w: remove invalid object", ErrDependency)
		}
		return nil, err
	}
	return metadata, nil
}

func (logic *Logic) requireAccess(ctx context.Context, environmentID, projectID int64) error {
	allowed, err := logic.authorizer.HasEnvironmentPermission(ctx, environmentID, PermissionAssetWrite)
	if err != nil {
		return fmt.Errorf("%w: check environment permission", ErrDependency)
	}
	if !allowed {
		return fmt.Errorf("%w", ErrForbidden)
	}
	exists, err := logic.projects.ProjectExists(ctx, environmentID, projectID)
	if err != nil {
		return fmt.Errorf("%w: check project scope", ErrDependency)
	}
	if !exists {
		return fmt.Errorf("%w", ErrProjectNotFound)
	}
	return nil
}

func (logic *Logic) validateUpload(filename, contentType string, size uint64) (string, string, error) {
	if filename == "" || len(filename) > 255 || strings.TrimSpace(filename) != filename || strings.ContainsAny(filename, "/\\\x00\r\n") {
		return "", "", fmt.Errorf("%w: filename is invalid", ErrInvalidInput)
	}
	for _, character := range filename {
		if unicode.IsControl(character) {
			return "", "", fmt.Errorf("%w: filename contains control characters", ErrInvalidInput)
		}
	}
	if filename == "." || filename == ".." {
		return "", "", fmt.Errorf("%w: filename is invalid", ErrInvalidInput)
	}
	extension := strings.ToLower(path.Ext(filename))
	allowedTypes, ok := logic.policy.AllowedByExt[extension]
	if !ok {
		return "", "", fmt.Errorf("%w: file extension is not allowed", ErrInvalidInput)
	}
	if size == 0 || size > logic.policy.MaxSize {
		return "", "", fmt.Errorf("%w: file size exceeds upload policy", ErrInvalidInput)
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || strings.TrimSpace(contentType) != contentType || strings.ContainsAny(contentType, "\r\n") {
		return "", "", fmt.Errorf("%w: content type is invalid", ErrInvalidInput)
	}
	mediaType = strings.ToLower(mediaType)
	for _, allowed := range allowedTypes {
		if mediaType == allowed {
			return extension, mediaType, nil
		}
	}
	return "", "", fmt.Errorf("%w: MIME type does not match file extension", ErrInvalidInput)
}

func (logic *Logic) validateCompleted(extension string, metadata *object.Metadata) error {
	if metadata == nil || metadata.Size == 0 || metadata.Size > logic.policy.MaxSize {
		return fmt.Errorf("%w: completed asset size is invalid", ErrAssetIntegrity)
	}
	allowedTypes, ok := logic.policy.AllowedByExt[extension]
	if !ok {
		return fmt.Errorf("%w: completed asset extension is invalid", ErrAssetIntegrity)
	}
	mediaType, _, err := mime.ParseMediaType(metadata.ContentType)
	if err != nil {
		return fmt.Errorf("%w: completed asset MIME type is invalid", ErrAssetIntegrity)
	}
	for _, allowed := range allowedTypes {
		if strings.EqualFold(mediaType, allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: completed asset MIME type does not match its extension", ErrAssetIntegrity)
}

func validatePolicy(policy Policy) error {
	if policy.MaxSize == 0 || policy.UploadLifetime <= 0 || policy.UploadLifetime > 24*time.Hour || len(policy.AllowedByExt) == 0 {
		return fmt.Errorf("asset upload policy is invalid")
	}
	for extension, contentTypes := range policy.AllowedByExt {
		if extension == "" || extension != strings.ToLower(extension) || !strings.HasPrefix(extension, ".") || len(contentTypes) == 0 {
			return fmt.Errorf("asset upload policy contains an invalid extension")
		}
		for _, contentType := range contentTypes {
			mediaType, _, err := mime.ParseMediaType(contentType)
			if err != nil || mediaType != strings.ToLower(contentType) {
				return fmt.Errorf("asset upload policy contains an invalid MIME type")
			}
		}
	}
	return nil
}
