package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestPermissionReplacementRejectsInvalidIDsWithoutClearingGrants(t *testing.T) {
	tests := []struct {
		name           string
		permissionIDs  []int64
		permissionRows *sqlmock.Rows
	}{
		{
			name:          "duplicate",
			permissionIDs: []int64{5, 5},
			permissionRows: smokeModelRows(t, &model.Permission{
				RecordMeta: model.RecordMeta{ID: 5},
				Key:        "config:read",
			}),
		},
		{
			name:           "nonpositive",
			permissionIDs:  []int64{0},
			permissionRows: sqlmock.NewRows(new(model.Permission).LormModelDescriptor().AllFields()),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage, mock := newSmokeEngine(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `roles` WHERE .*`deleted_at` IS NULL.*FOR UPDATE").
				WillReturnRows(smokeModelRows(t, &model.Role{Meta: model.Meta{ID: 3}}))
			mock.ExpectQuery("SELECT .* FROM `permissions` WHERE .*FOR UPDATE").
				WillReturnRows(test.permissionRows)
			mock.ExpectRollback()
			err := storage.CoreDB.TX(context.Background(), func(ctx context.Context) error {
				_, err := NewRoleRepository(storage).UpdatePermissions(ctx, 3, test.permissionIDs, 9, &model.AuditLog{})
				return err
			})
			require.ErrorIs(t, err, ErrPermissionSetMismatch)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
