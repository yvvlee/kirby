package object

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const localMetadataSuffix = ".kirby-metadata"

type localUploadClaims struct {
	Key         string `json:"key"`
	ContentType string `json:"content_type"`
	Size        uint64 `json:"size"`
	ExpiresAt   int64  `json:"expires_at"`
}

type localMetadata struct {
	ContentType         string    `json:"content_type"`
	DeclaredContentType string    `json:"declared_content_type"`
	DeclaredSize        uint64    `json:"declared_size"`
	CreatedAt           time.Time `json:"created_at"`
}

// LocalStorage stores assets on one server. It also serves the direct-upload
// and public-object paths that BE-10 registers on the management HTTP server.
type LocalStorage struct {
	root       string
	signingKey []byte
	now        func() time.Time
}

// NewLocal creates a local adapter and a process-local upload signing key.
func NewLocal(directory string) (*LocalStorage, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate local upload signing key: %w", err)
	}
	return newLocal(directory, key, time.Now)
}

func newLocal(directory string, signingKey []byte, now func() time.Time) (*LocalStorage, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("local object storage directory is required")
	}
	if len(signingKey) < 32 {
		return nil, fmt.Errorf("local upload signing key must contain at least 32 bytes")
	}
	if now == nil {
		return nil, fmt.Errorf("local object storage clock is nil")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve local object storage directory")
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("create local object storage directory")
	}
	root, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve local object storage directory")
	}
	return &LocalStorage{
		root:       root,
		signingKey: append([]byte(nil), signingKey...),
		now:        now,
	}, nil
}

// PresignUpload returns a relative URL so credentials and internal endpoints
// are never embedded in the management response.
func (s *LocalStorage) PresignUpload(ctx context.Context, input PresignUploadInput) (*UploadTicket, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	contentType, err := validatePresignInput(input)
	if err != nil {
		return nil, err
	}
	expiresAt := s.now().UTC().Add(input.ExpiresIn)
	claims := localUploadClaims{
		Key:         input.Key,
		ContentType: contentType,
		Size:        input.Size,
		ExpiresAt:   expiresAt.Unix(),
	}
	token, err := s.signClaims(claims)
	if err != nil {
		return nil, fmt.Errorf("sign local upload ticket: %w", ErrStorageUnavailable)
	}
	return &UploadTicket{
		Key: input.Key,
		URL: LocalUploadPath + "?token=" + token,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		ExpiresAt: expiresAt,
	}, nil
}

// CompleteUpload reads and verifies the actual file and its signed upload
// declaration.
func (s *LocalStorage) CompleteUpload(ctx context.Context, key string) (*Metadata, error) {
	metadata, err := s.ReadMetadata(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := validateCompletedMetadata(metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

// ReadMetadata reads metadata without trusting a browser-supplied value.
func (s *LocalStorage) ReadMetadata(ctx context.Context, key string) (*Metadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	objectPath, err := s.safePath(key, false)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(objectPath)
	if err != nil {
		return nil, classifyLocalReadError(err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: local object is not a regular file", ErrObjectIntegrity)
	}
	metadataPath := objectPath + localMetadataSuffix
	metadataInfo, err := os.Lstat(metadataPath)
	if err != nil {
		return nil, classifyLocalReadError(err)
	}
	if !metadataInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: local metadata is not a regular file", ErrObjectIntegrity)
	}
	encoded, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, classifyLocalReadError(err)
	}
	var stored localMetadata
	decoderErr := json.Unmarshal(encoded, &stored)
	if decoderErr != nil {
		return nil, fmt.Errorf("%w: local metadata cannot be decoded", ErrObjectIntegrity)
	}
	if info.Size() < 0 {
		return nil, fmt.Errorf("%w: local object size is invalid", ErrObjectIntegrity)
	}
	return &Metadata{
		Key:                 key,
		URL:                 LocalObjectPathPrefix + key,
		ContentType:         stored.ContentType,
		Size:                uint64(info.Size()),
		DeclaredContentType: stored.DeclaredContentType,
		DeclaredSize:        stored.DeclaredSize,
		LastModified:        info.ModTime().UTC(),
	}, nil
}

// DeleteIncomplete removes both the object and its private metadata sidecar.
func (s *LocalStorage) DeleteIncomplete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	objectPath, err := s.safePath(key, false)
	if err != nil {
		return err
	}
	var deleteErrors []error
	for _, target := range []string{objectPath, objectPath + localMetadataSuffix} {
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			deleteErrors = append(deleteErrors, fmt.Errorf("remove local object data: %w", ErrStorageUnavailable))
		}
	}
	return errors.Join(deleteErrors...)
}

// Close clears the in-memory signing key.
func (s *LocalStorage) Close() error {
	for index := range s.signingKey {
		s.signingKey[index] = 0
	}
	return nil
}

// ServeHTTP handles only the two explicit local-storage routes.
func (s *LocalStorage) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch {
	case request.URL.Path == LocalUploadPath:
		s.handleUpload(writer, request)
	case strings.HasPrefix(request.URL.Path, LocalObjectPathPrefix):
		s.handleObject(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (s *LocalStorage) handleUpload(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		writer.Header().Set("Allow", http.MethodPut)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request.Header.Get("Authorization") != "" || request.Header.Get("X-Kirby-API-Key") != "" || request.Header.Get("Cookie") != "" {
		http.Error(writer, "management credentials are not accepted by object storage", http.StatusBadRequest)
		return
	}
	claims, err := s.verifyToken(request.URL.Query().Get("token"))
	if err != nil {
		http.Error(writer, "invalid or expired upload ticket", http.StatusUnauthorized)
		return
	}
	requestType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || strings.ToLower(requestType) != claims.ContentType {
		http.Error(writer, "content type does not match upload ticket", http.StatusBadRequest)
		return
	}
	if request.ContentLength >= 0 && uint64(request.ContentLength) != claims.Size {
		http.Error(writer, "content length does not match upload ticket", http.StatusBadRequest)
		return
	}
	if err := s.writeUpload(request.Context(), claims, request.Body); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrObjectIntegrity) {
			status = http.StatusBadRequest
		}
		http.Error(writer, http.StatusText(status), status)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *LocalStorage) handleObject(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(request.URL.Path, LocalObjectPathPrefix)
	metadata, err := s.CompleteUpload(request.Context(), key)
	if err != nil {
		if errors.Is(err, ErrObjectNotFound) || errors.Is(err, ErrInvalidInput) {
			http.NotFound(writer, request)
			return
		}
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	objectPath, err := s.safePath(key, false)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", metadata.ContentType)
	writer.Header().Set("Content-Length", strconv.FormatUint(metadata.Size, 10))
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(writer, request, objectPath)
}

func (s *LocalStorage) writeUpload(ctx context.Context, claims localUploadClaims, source io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	objectPath, err := s.safePath(claims.Key, true)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(objectPath); err == nil {
		return fmt.Errorf("%w: object already exists", ErrObjectIntegrity)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect local object: %w", ErrStorageUnavailable)
	}
	temporary, err := os.CreateTemp(filepath.Dir(objectPath), ".kirby-upload-*")
	if err != nil {
		return fmt.Errorf("create local upload: %w", ErrStorageUnavailable)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o640); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure local upload: %w", ErrStorageUnavailable)
	}
	written, copyErr := io.Copy(temporary, io.LimitReader(source, int64(claims.Size)+1))
	closeErr := temporary.Close()
	if copyErr != nil || closeErr != nil {
		return fmt.Errorf("write local upload: %w", ErrStorageUnavailable)
	}
	if written < 0 || uint64(written) != claims.Size {
		return fmt.Errorf("%w: uploaded size does not match declaration", ErrObjectIntegrity)
	}
	if err := os.Link(temporaryPath, objectPath); err != nil {
		return fmt.Errorf("publish local upload: %w", ErrStorageUnavailable)
	}
	stored := localMetadata{
		ContentType:         claims.ContentType,
		DeclaredContentType: claims.ContentType,
		DeclaredSize:        claims.Size,
		CreatedAt:           s.now().UTC(),
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		_ = os.Remove(objectPath)
		return fmt.Errorf("encode local upload metadata: %w", ErrStorageUnavailable)
	}
	if err := writeExclusiveFile(objectPath+localMetadataSuffix, encoded, 0o640); err != nil {
		_ = os.Remove(objectPath)
		return fmt.Errorf("publish local upload metadata: %w", ErrStorageUnavailable)
	}
	return nil
}

func (s *LocalStorage) safePath(key string, createParents bool) (string, error) {
	if _, err := ParseObjectKey(key); err != nil {
		return "", err
	}
	candidate := filepath.Join(s.root, filepath.FromSlash(key))
	relative, err := filepath.Rel(s.root, candidate)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("%w: object path escapes storage root", ErrInvalidInput)
	}
	parent := filepath.Dir(candidate)
	if createParents {
		if err := s.createSafeDirectories(parent); err != nil {
			return "", err
		}
	} else if err := s.rejectSymlinkPath(parent); err != nil {
		return "", err
	}
	return candidate, nil
}

func (s *LocalStorage) createSafeDirectories(directory string) error {
	relative, err := filepath.Rel(s.root, directory)
	if err != nil {
		return fmt.Errorf("%w: local object directory is invalid", ErrInvalidInput)
	}
	current := s.root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o750); err != nil && !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("create local object directory: %w", ErrStorageUnavailable)
			}
			info, err = os.Lstat(current)
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: local object directory is unsafe", ErrObjectIntegrity)
		}
	}
	return nil
}

func (s *LocalStorage) rejectSymlinkPath(directory string) error {
	relative, err := filepath.Rel(s.root, directory)
	if err != nil {
		return fmt.Errorf("%w: local object directory is invalid", ErrInvalidInput)
	}
	current := s.root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w", ErrObjectNotFound)
		}
		if err != nil {
			return fmt.Errorf("read local object directory: %w", ErrStorageUnavailable)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: local object directory is unsafe", ErrObjectIntegrity)
		}
	}
	return nil
}

func (s *LocalStorage) signClaims(claims localUploadClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signature := hmac.New(sha256.New, s.signingKey)
	_, _ = signature.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature.Sum(nil)), nil
}

func (s *LocalStorage) verifyToken(token string) (localUploadClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return localUploadClaims{}, fmt.Errorf("invalid upload ticket")
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return localUploadClaims{}, fmt.Errorf("invalid upload ticket")
	}
	expectedSignature := hmac.New(sha256.New, s.signingKey)
	_, _ = expectedSignature.Write([]byte(parts[0]))
	if !hmac.Equal(providedSignature, expectedSignature.Sum(nil)) {
		return localUploadClaims{}, fmt.Errorf("invalid upload ticket")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return localUploadClaims{}, fmt.Errorf("invalid upload ticket")
	}
	var claims localUploadClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return localUploadClaims{}, fmt.Errorf("invalid upload ticket")
	}
	if _, err := validatePresignInput(PresignUploadInput{
		Key: claims.Key, ContentType: claims.ContentType, Size: claims.Size, ExpiresIn: time.Second,
	}); err != nil {
		return localUploadClaims{}, fmt.Errorf("invalid upload ticket")
	}
	if !s.now().Before(time.Unix(claims.ExpiresAt, 0)) {
		return localUploadClaims{}, fmt.Errorf("expired upload ticket")
	}
	return claims, nil
}

func writeExclusiveFile(path string, contents []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func classifyLocalReadError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w", ErrObjectNotFound)
	}
	return fmt.Errorf("read local object: %w", ErrStorageUnavailable)
}
