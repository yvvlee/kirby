package object

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testObjectID = "123e4567-e89b-12d3-a456-426614174000"

func TestBuildAndParseObjectKey(t *testing.T) {
	key, err := BuildObjectKey(12, 34, testObjectID, ".png")
	require.NoError(t, err)
	assert.Equal(t, "environments/12/projects/34/assets/"+testObjectID+".png", key)

	scope, err := ParseObjectKey(key)
	require.NoError(t, err)
	assert.Equal(t, int64(12), scope.EnvironmentID)
	assert.Equal(t, int64(34), scope.ProjectID)
	assert.Equal(t, testObjectID, scope.ObjectID)
	assert.Equal(t, ".png", scope.Extension)
}

func TestParseObjectKeyRejectsTraversalAndForgedShapes(t *testing.T) {
	for _, key := range []string{
		"../environments/1/projects/2/assets/" + testObjectID + ".png",
		"environments/1/projects/2/assets/../../secret.png",
		"environments/01/projects/2/assets/" + testObjectID + ".png",
		"environments/1/projects/2/assets/not-a-uuid.png",
		"environments/1/projects/2/assets/" + testObjectID + ".PNG",
		"/environments/1/projects/2/assets/" + testObjectID + ".png",
		"environments\\1\\projects\\2\\assets\\" + testObjectID + ".png",
	} {
		t.Run(key, func(t *testing.T) {
			_, err := ParseObjectKey(key)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidInput))
		})
	}
}
