package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestCreateInitializesVersionAndTimestamps(t *testing.T) {
	storage, mock := newSmokeEngine(t)
	mock.ExpectExec("INSERT INTO `projects`").WillReturnResult(sqlmock.NewResult(41, 1))
	project := &model.Project{
		Key: "application",
	}
	require.NoError(t, NewProjectRepository(storage).Create(context.Background(), 0, project))
	require.EqualValues(t, 41, project.ID)
	require.EqualValues(t, 1, project.Version)
	require.False(t, project.CreatedAt.IsZero())
	require.False(t, project.UpdatedAt.IsZero())
	require.Nil(t, project.DeletedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigRestoreResetsChildrenInsideTransaction(t *testing.T) {
	storage, mock := newSmokeEngine(t)
	deletedAt := time.Now().Add(-time.Hour)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT p.* FROM projects AS p .* FOR UPDATE").
		WillReturnRows(smokeModelRows(t, &model.Project{
			Meta: model.Meta{
				ID: 3,
			},
		}))
	mock.ExpectQuery("SELECT .* FROM `configs` WHERE `key` = \\? AND `project_id` = \\? LIMIT 1 FOR UPDATE").
		WithArgs("feature", int64(3)).
		WillReturnRows(smokeModelRows(t, &model.Config{
			Meta: model.Meta{
				ID:        42,
				Version:   8,
				DeletedAt: &deletedAt,
			},
			ProjectID: 3,
			Key:       "feature",
		}))
	mock.ExpectExec("UPDATE `structures` SET .* WHERE `deleted_at` IS NULL AND `config_id` = \\?").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE `config_enums` SET .* WHERE `deleted_at` IS NULL AND `config_id` = \\?").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `configs` SET .*`deleted_at` = \\?.* WHERE `id` = \\?$").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	config := &model.Config{
		ProjectID: 3,
		Key:       "feature",
		TypeJSON:  `{"baseType":"STRING"}`,
		Value:     `""`,
	}
	err := storage.CoreDB.TX(context.Background(), func(ctx context.Context) error {
		return NewConfigRepository(storage).Create(ctx, 5, 3, config)
	})
	require.NoError(t, err)
	require.EqualValues(t, 42, config.ID)
	require.EqualValues(t, 9, config.Version)
	require.Nil(t, config.DeletedAt)
	require.False(t, config.CreatedAt.IsZero())
	require.Equal(t, config.CreatedAt, config.UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigUpdateRejectsStaleVersion(t *testing.T) {
	storage, mock := newSmokeEngine(t)
	mock.ExpectQuery("SELECT c.* FROM configs AS c .*`c`.`deleted_at` IS NULL.*").
		WillReturnRows(smokeModelRows(t, &model.Config{
			Meta: model.Meta{
				ID:      42,
				Version: 4,
			},
		}))
	mock.ExpectExec("UPDATE `configs` SET .* WHERE `deleted_at` IS NULL AND `id` = \\? AND `version` = \\?").
		WillReturnResult(sqlmock.NewResult(0, 0))
	err := NewConfigRepository(storage).UpdateValue(context.Background(), 5, 42, ConfigValueUpdate{
		Value:   `"changed"`,
		Version: 3,
	})
	require.ErrorIs(t, err, ErrNoRowsAffected)
	require.NoError(t, mock.ExpectationsWereMet())
}
