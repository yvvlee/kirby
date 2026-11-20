package repository

import (
	"context"
	"testing"

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
	_, err := NewProjectRepository(storage).Create(context.Background(), 0, project)
	require.NoError(t, err)
	require.EqualValues(t, 41, project.ID)
	require.EqualValues(t, 0, project.Version)
	require.False(t, project.CreatedAt.IsZero())
	require.False(t, project.UpdatedAt.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigDeleteRemovesChildrenAndRecreateInsertsFreshRow(t *testing.T) {
	storage, mock := newSmokeEngine(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT p.* FROM projects AS p .* FOR UPDATE").
		WillReturnRows(smokeModelRows(t, &model.Project{
			Meta: model.Meta{
				ID: 3,
			},
		}))
	mock.ExpectExec("INSERT INTO `configs`").WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectQuery("SELECT c.* FROM configs AS c .* WHERE `c`.`id` = \\? AND `e`.`id` = \\? LIMIT 1").
		WithArgs(int64(42), int64(5)).
		WillReturnRows(smokeModelRows(t, &model.Config{
			Meta:      model.Meta{ID: 42},
			ProjectID: 3,
			Key:       "feature",
		}))
	mock.ExpectExec("DELETE FROM `structures` WHERE `config_id` = \\?").
		WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM `config_enums` WHERE `config_id` = \\?").
		WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `configs` WHERE `id` = \\?").
		WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT p.* FROM projects AS p .* FOR UPDATE").
		WillReturnRows(smokeModelRows(t, &model.Project{
			Meta: model.Meta{
				ID: 3,
			},
		}))
	mock.ExpectExec("INSERT INTO `configs`").WillReturnResult(sqlmock.NewResult(43, 1))
	mock.ExpectCommit()
	repository := NewConfigRepository(storage)
	original := &model.Config{
		ProjectID: 3,
		Key:       "feature",
		TypeJSON:  `{"baseType":"STRING"}`,
		Value:     `""`,
	}
	replacement := &model.Config{
		ProjectID: 3,
		Key:       "feature",
		TypeJSON:  `{"baseType":"STRING"}`,
		Value:     `""`,
	}
	err := storage.CoreDB.TX(context.Background(), func(ctx context.Context) error {
		if _, err := repository.Create(ctx, 5, 3, original); err != nil {
			return err
		}
		if _, err := repository.Delete(ctx, 5, original.ID); err != nil {
			return err
		}
		_, err := repository.Create(ctx, 5, 3, replacement)
		return err
	})
	require.NoError(t, err)
	require.EqualValues(t, 42, original.ID)
	require.EqualValues(t, 0, original.Version)
	require.EqualValues(t, 43, replacement.ID)
	require.NotEqual(t, original.ID, replacement.ID)
	require.EqualValues(t, 0, replacement.Version)
	require.False(t, replacement.CreatedAt.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigUpdateRejectsStaleVersion(t *testing.T) {
	storage, mock := newSmokeEngine(t)
	mock.ExpectQuery("SELECT c.* FROM configs AS c .*").
		WillReturnRows(smokeModelRows(t, &model.Config{
			Meta: model.Meta{
				ID:      42,
				Version: 4,
			},
		}))
	mock.ExpectExec("UPDATE `configs` SET .* WHERE `id` = \\? AND `version` = \\?").
		WillReturnResult(sqlmock.NewResult(0, 0))
	_, err := NewConfigRepository(storage).UpdateValue(context.Background(), 5, 42, ConfigValueUpdate{
		Value:   `"changed"`,
		Version: 3,
	})
	require.ErrorIs(t, err, ErrNoRowsAffected)
	require.NoError(t, mock.ExpectationsWereMet())
}
