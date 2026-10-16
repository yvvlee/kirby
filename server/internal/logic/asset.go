package logic

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

	"github.com/yvvlee/kirby/server/internal/infra/object"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
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

// assetAuthorizer checks the caller's role in one environment.
type assetAuthorizer interface {
	HasEnvironmentPermission(ctx context.Context, environmentID int64, key string) (bool, error)
}

// assetProjectScope checks that a project belongs to the selected environment.
type assetProjectScope interface {
	ProjectExists(ctx context.Context, environmentID int64, projectID int64) (bool, error)
}

// Policy controls accepted extensions, MIME types, and size.
type Policy struct {
	MaxSize        uint64
	AllowedByExt   map[string][]string
	UploadLifetime time.Duration
}

// DefaultPolicy permits common image, media, document, and archive formats.
// Parameters: none.
// Returns: an independent copy of the accepted upload types and limits.
// Throws: none.
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

// AssetLogic coordinates authorization, project ownership, policy, and storage.
type AssetLogic struct {
	storage    object.ObjectStorage
	authorizer assetAuthorizer
	projects   assetProjectScope
	policy     Policy
	newID      func() (string, error)
}

// NewAssetLogic constructs asset logic with the fixed public upload policy.
// Parameters: storage publishes assets; authorizer checks the caller; projects resolves project scope.
// Returns: upload logic with the fixed server policy and cryptographic object IDs.
// Throws: an error if the built-in upload policy is inconsistent.
func NewAssetLogic(
	storage object.ObjectStorage,
	authorizer assetAuthorizer,
	projects assetProjectScope,
) (*AssetLogic, error) {
	return newLogic(storage, authorizer, projects, DefaultPolicy(), func() (string, error) {
		value, err := uuid.NewRandom()
		return value.String(), err
	})
}

func newLogic(storage object.ObjectStorage, authorizer assetAuthorizer, projects assetProjectScope, policy Policy, newID func() (string, error)) (*AssetLogic, error) {
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	return &AssetLogic{
		storage:    storage,
		authorizer: authorizer,
		projects:   projects,
		policy:     policy,
		newID:      newID,
	}, nil
}

// Presign authorizes the request and creates a scoped, unguessable key.
// Parameters: ctx carries the caller; input describes the requested project upload.
// Returns: a scoped ticket with its expiration and upload requirements.
// Throws: invalid input, forbidden access, missing project, or wrapped dependency errors.
func (l *AssetLogic) Presign(ctx context.Context, input PresignInput) (*object.UploadTicket, error) {
	extension, contentType, err := l.validateUpload(input.Filename, input.ContentType, input.Size)
	if err != nil {
		return nil, err
	}
	if err := l.requireAccess(ctx, input.EnvironmentID, input.ProjectID); err != nil {
		return nil, err
	}
	objectID, err := l.newID()
	if err != nil {
		return nil, fmt.Errorf("%w: generate object ID: %w", ErrDependency, err)
	}
	key, err := object.BuildObjectKey(input.EnvironmentID, input.ProjectID, objectID, extension)
	if err != nil {
		return nil, fmt.Errorf("%w: generated object key is invalid: %w", ErrDependency, err)
	}
	ticket, err := l.storage.PresignUpload(ctx, object.PresignUploadInput{
		Key:         key,
		ContentType: contentType,
		Size:        input.Size,
		ExpiresIn:   l.policy.UploadLifetime,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: create upload ticket: %w", ErrDependency, err)
	}
	return ticket, nil
}

// Complete validates key ownership before querying provider metadata.
// Parameters: ctx carries the caller; environmentID and projectID identify ownership; key identifies the upload.
// Returns: validated metadata for the published object.
// Throws: ownership, permission, integrity, conflict, missing-object, or wrapped dependency errors.
func (l *AssetLogic) Complete(ctx context.Context, environmentID, projectID int64, key string) (*object.Metadata, error) {
	scope, err := object.ParseObjectKey(key)
	if err != nil {
		return nil, fmt.Errorf("%w: object key is invalid", ErrInvalidInput)
	}
	if scope.EnvironmentID != environmentID || scope.ProjectID != projectID {
		return nil, fmt.Errorf("%w: object belongs to another environment or project", ErrForbidden)
	}
	if err := l.requireAccess(ctx, environmentID, projectID); err != nil {
		return nil, err
	}
	metadata, err := l.completeObject(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := l.validatePublication(ctx, scope, metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

func (l *AssetLogic) completeObject(ctx context.Context, key string) (*object.Metadata, error) {
	metadata, err := l.storage.CompleteUpload(ctx, key)
	if err == nil {
		return metadata, nil
	}
	switch {
	case errors.Is(err, object.ErrObjectNotFound):
		return nil, fmt.Errorf("%w: %w", ErrAssetNotFound, err)
	case errors.Is(err, object.ErrObjectIntegrity):
		if deleteErr := l.storage.DeleteIncomplete(ctx, key); deleteErr != nil {
			return nil, fmt.Errorf("%w: remove invalid object: %w", ErrDependency, errors.Join(err, deleteErr))
		}
		return nil, fmt.Errorf("%w: stored metadata differs from upload declaration: %w", ErrAssetIntegrity, err)
	case errors.Is(err, object.ErrObjectConflict):
		return nil, fmt.Errorf("%w: %w", ErrAssetConflict, err)
	default:
		return nil, fmt.Errorf("%w: inspect uploaded object: %w", ErrDependency, err)
	}
}

func (l *AssetLogic) validatePublication(ctx context.Context, scope object.ObjectScope, metadata *object.Metadata) error {
	publishedScope, err := object.ParseObjectKey(metadata.Key)
	if err != nil || publishedScope.Temporary ||
		publishedScope.EnvironmentID != scope.EnvironmentID || publishedScope.ProjectID != scope.ProjectID ||
		publishedScope.Extension != scope.Extension {
		return fmt.Errorf("%w: storage returned an invalid published object scope", ErrAssetIntegrity)
	}
	if err := l.validateCompleted(scope.Extension, metadata); err != nil {
		if deleteErr := l.storage.DeleteIncomplete(ctx, metadata.Key); deleteErr != nil {
			return fmt.Errorf("%w: remove invalid object: %w", ErrDependency, errors.Join(err, deleteErr))
		}
		return err
	}
	return nil
}

func (l *AssetLogic) requireAccess(ctx context.Context, environmentID, projectID int64) error {
	allowed, err := l.authorizer.HasEnvironmentPermission(ctx, environmentID, PermissionAssetWrite)
	if err != nil {
		return fmt.Errorf("%w: check environment permission: %w", ErrDependency, err)
	}
	if !allowed {
		return fmt.Errorf("%w", ErrForbidden)
	}
	exists, err := l.projects.ProjectExists(ctx, environmentID, projectID)
	if err != nil {
		return fmt.Errorf("%w: check project scope: %w", ErrDependency, err)
	}
	if !exists {
		return fmt.Errorf("%w", ErrProjectNotFound)
	}
	return nil
}

func (l *AssetLogic) validateUpload(filename, contentType string, size uint64) (string, string, error) {
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
	allowedTypes, ok := l.policy.AllowedByExt[extension]
	if !ok {
		return "", "", fmt.Errorf("%w: file extension is not allowed", ErrInvalidInput)
	}
	if size == 0 || size > l.policy.MaxSize {
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

func (l *AssetLogic) validateCompleted(extension string, metadata *object.Metadata) error {
	if metadata.Size == 0 || metadata.Size > l.policy.MaxSize {
		return fmt.Errorf("%w: completed asset size is invalid", ErrAssetIntegrity)
	}
	allowedTypes, ok := l.policy.AllowedByExt[extension]
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
		if err := validatePolicyMIMETypes(contentTypes); err != nil {
			return err
		}
	}
	return nil
}

func validatePolicyMIMETypes(contentTypes []string) error {
	for _, contentType := range contentTypes {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != strings.ToLower(contentType) {
			return fmt.Errorf("asset upload policy contains an invalid MIME type")
		}
	}
	return nil
}

type assetPermissionAdapter struct{ resolver *permission.Resolver }

// HasEnvironmentPermission connects asset operations to their scoped dependencies.
// Parameters: ctx identifies the actor; environmentID selects the scope; key is the permission.
// Returns: whether the actor has the permission.
// Throws: identity or permission resolution errors.
func (a assetPermissionAdapter) HasEnvironmentPermission(ctx context.Context, environmentID int64, key string) (bool, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return false, err
	}
	err = a.resolver.Require(ctx, actor.UserID, environmentID, key)
	if errors.Is(err, permission.ErrForbidden) {
		return false, nil
	}
	return err == nil, err
}

type projectScope struct {
	projects repository.ProjectRepository
}

// ProjectExists connects asset operations to their scoped dependencies.
// Parameters: ctx controls the lookup; environmentID and projectID identify the scope.
// Returns: whether an active project exists in that scope.
// Throws: database errors; a missing project returns false.
func (p projectScope) ProjectExists(ctx context.Context, environmentID, projectID int64) (bool, error) {
	_, err := p.projects.FindByID(ctx, environmentID, projectID)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// ProvideAssetLogic connects asset operations to their scoped dependencies.
// Parameters: objects stores uploads; resolver authorizes actors; projects verifies project ownership.
// Returns: the asset logic with adapters bound to the shared authorization and repository implementations.
// Throws: upload policy construction errors.
func ProvideAssetLogic(
	objects object.ObjectStorage,
	resolver *permission.Resolver,
	projects repository.ProjectRepository,
) (*AssetLogic, error) {
	return NewAssetLogic(objects, assetPermissionAdapter{resolver: resolver}, projectScope{projects: projects})
}
