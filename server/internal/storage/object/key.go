package object

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// ObjectScope is encoded in every object key.
type ObjectScope struct {
	EnvironmentID int64
	ProjectID     int64
	ObjectID      string
	Extension     string
}

// BuildObjectKey creates a key whose ownership can be verified without using
// the user-provided filename as a path.
func BuildObjectKey(environmentID, projectID int64, objectID, extension string) (string, error) {
	if environmentID <= 0 || projectID <= 0 {
		return "", fmt.Errorf("%w: environment and project IDs must be positive", ErrInvalidInput)
	}
	parsedID, err := uuid.Parse(objectID)
	if err != nil || parsedID.String() != objectID {
		return "", fmt.Errorf("%w: object ID must be a canonical UUID", ErrInvalidInput)
	}
	if len(extension) < 2 || extension[0] != '.' || extension != strings.ToLower(extension) {
		return "", fmt.Errorf("%w: extension must be lowercase and start with a dot", ErrInvalidInput)
	}
	for _, character := range extension[1:] {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return "", fmt.Errorf("%w: extension contains unsupported characters", ErrInvalidInput)
		}
	}
	return fmt.Sprintf("environments/%d/projects/%d/assets/%s%s", environmentID, projectID, objectID, extension), nil
}

// ParseObjectKey validates the exact canonical key shape.
func ParseObjectKey(key string) (ObjectScope, error) {
	if key == "" || len(key) > 512 || strings.ContainsAny(key, "\\\x00\r\n") || path.Clean(key) != key || strings.HasPrefix(key, "/") {
		return ObjectScope{}, fmt.Errorf("%w: object key is not canonical", ErrInvalidInput)
	}
	parts := strings.Split(key, "/")
	if len(parts) != 6 || parts[0] != "environments" || parts[2] != "projects" || parts[4] != "assets" {
		return ObjectScope{}, fmt.Errorf("%w: object key has an invalid scope", ErrInvalidInput)
	}
	environmentID, err := parseCanonicalPositiveID(parts[1])
	if err != nil {
		return ObjectScope{}, err
	}
	projectID, err := parseCanonicalPositiveID(parts[3])
	if err != nil {
		return ObjectScope{}, err
	}
	extension := path.Ext(parts[5])
	objectID := strings.TrimSuffix(parts[5], extension)
	parsedID, err := uuid.Parse(objectID)
	if err != nil || parsedID.String() != objectID || extension == "" || extension != strings.ToLower(extension) {
		return ObjectScope{}, fmt.Errorf("%w: object filename is invalid", ErrInvalidInput)
	}
	for _, character := range extension[1:] {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return ObjectScope{}, fmt.Errorf("%w: object extension is invalid", ErrInvalidInput)
		}
	}
	return ObjectScope{
		EnvironmentID: environmentID,
		ProjectID:     projectID,
		ObjectID:      objectID,
		Extension:     extension,
	}, nil
}

func parseCanonicalPositiveID(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != value {
		return 0, fmt.Errorf("%w: object scope ID is invalid", ErrInvalidInput)
	}
	return parsed, nil
}
