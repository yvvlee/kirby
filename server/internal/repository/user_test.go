package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestLastEnabledSystemAdministratorCannotBeRemoved(t *testing.T) {
	admins := []model.User{{
		Meta:          model.Meta{ID: 7},
		Enabled:       true,
		IsSystemAdmin: true,
	}}
	if err := ensureSystemAdminSurvives(admins, 7); !errors.Is(err, ErrLastSystemAdmin) {
		t.Fatalf("last administrator removal error = %v", err)
	}

	admins = append(admins, model.User{
		Meta:          model.Meta{ID: 8},
		Enabled:       true,
		IsSystemAdmin: true,
	})
	if err := ensureSystemAdminSurvives(admins, 7); err != nil {
		t.Fatalf("removing one of two administrators failed: %v", err)
	}
}

func TestDisabledOrDeletedAdministratorsDoNotSatisfyProtection(t *testing.T) {
	deletedAt := time.Now()
	admins := []model.User{
		{
			Meta:          model.Meta{ID: 1},
			Enabled:       true,
			IsSystemAdmin: true,
		},
		{
			Meta:          model.Meta{ID: 2},
			Enabled:       false,
			IsSystemAdmin: true,
		},
		{
			Meta: model.Meta{
				ID:        3,
				DeletedAt: &deletedAt,
			},
			Enabled:       true,
			IsSystemAdmin: true,
		},
	}
	if err := ensureSystemAdminSurvives(admins, 1); !errors.Is(err, ErrLastSystemAdmin) {
		t.Fatalf("unusable administrators bypassed protection: %v", err)
	}
}
