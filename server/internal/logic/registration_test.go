package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type registrationSettingsFake struct {
	repository.ApplicationSettingsRepository
	enabled bool
	err     error
}

func (s registrationSettingsFake) Get(context.Context) (*model.ApplicationSettings, error) {
	return &model.ApplicationSettings{AllowSelfRegistration: s.enabled}, s.err
}
func TestRegistrationDisabledDoesNotTouchUsersOrHasher(t *testing.T) {
	l := &AuthLogic{settings: registrationSettingsFake{enabled: false}}
	_, err := l.Register(context.Background(), "new", "New", "new@example.com", "long-password")
	require.ErrorIs(t, err, ErrRegistrationDisabled)
}
func TestRegistrationStatusPropagatesStorageFailure(t *testing.T) {
	expected := errors.New("database offline")
	l := NewSystemSettingsLogic(registrationSettingsFake{err: expected}, nil)
	_, err := l.RegistrationStatus(context.Background())
	require.ErrorIs(t, err, expected)
}
func TestRegistrationStatusReportsEnabledFlag(t *testing.T) {
	l := NewSystemSettingsLogic(registrationSettingsFake{enabled: true}, nil)
	enabled, err := l.RegistrationStatus(context.Background())
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestRegistrationReportsSettingsStorageFailure(t *testing.T) {
	failure := errors.New("database offline")
	l := &AuthLogic{settings: registrationSettingsFake{err: failure}}
	_, err := l.Register(context.Background(), "new", "New", "new@example.com", "long-password")
	require.ErrorIs(t, err, ErrAuthUnavailable)
	require.ErrorIs(t, err, failure)
}
