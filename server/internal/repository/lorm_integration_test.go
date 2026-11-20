package repository

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

// Real dialects catch joined scopes, nullable writes, RETURNING, and row-count
// semantics that SQL mocks cannot validate. Every case rolls back its fixture.
func TestLormRepositoriesIntegration(t *testing.T) {
	databases := []struct {
		name   string
		driver string
		dsnEnv string
	}{
		{
			name:   "mysql",
			driver: "mysql",
			dsnEnv: "KIRBY_TEST_MYSQL_DSN",
		},
		{
			name:   "postgres",
			driver: "pgx",
			dsnEnv: "KIRBY_TEST_PGSQL_DSN",
		},
	}
	for _, database := range databases {
		t.Run(database.name, func(t *testing.T) {
			runLormIntegrationDatabase(t, database.driver, database.dsnEnv)
		})
	}
}

func runLormIntegrationDatabase(t *testing.T, driver, dsnEnv string) {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skip("set " + dsnEnv + " to an initialized disposable database")
	}
	// One connection makes any repository that loses the transaction context
	// fail by timeout instead of silently committing outside the fixture.
	engine, err := lorm.NewEngine(driver, dsn, lorm.WithLogger(nil), lorm.WithMaxOpenConns(1))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, engine.Close()) })
	cases := []struct {
		name string
		run  func(*lormIntegrationFixture)
	}{
		{
			name: "sibling_environment",
			run:  verifyLormSiblingEnvironment,
		},
		{
			name: "project_environment",
			run:  verifyLormProjectEnvironment,
		},
		{
			name: "config",
			run:  verifyLormConfig,
		},
		{
			name: "structure",
			run:  verifyLormStructure,
		},
		{
			name: "enum",
			run:  verifyLormEnum,
		},
		{
			name: "snapshot",
			run:  verifyLormSnapshot,
		},
		{
			name: "api_key",
			run:  verifyLormAPIKey,
		},
		{
			name: "insert_timestamps",
			run:  verifyLormInsertTimestamps,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			runLormIntegrationTransaction(t, engine, test.run)
		})
	}
}

type lormIntegrationFixture struct {
	t           *testing.T
	ctx         context.Context
	storage     *infra.Storage
	project     *model.Project
	environment *model.Environment
	foreign     *model.Environment
	config      *model.Config
}

func runLormIntegrationTransaction(t *testing.T, engine *lorm.Engine, verify func(*lormIntegrationFixture)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rollback := errors.New("roll back integration fixture")
	var projectID int64
	err := engine.TX(ctx, func(txCtx context.Context) error {
		fixture := newLormIntegrationFixture(t, txCtx, engine)
		projectID = fixture.project.ID
		verify(fixture)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	_, found, err := engine.Query[*model.Project]().ID(projectID).Get(ctx)
	require.NoError(t, err)
	require.False(t, found, "fixture must not escape the transaction context")
}

func newLormIntegrationFixture(t *testing.T, ctx context.Context, engine *lorm.Engine) *lormIntegrationFixture {
	t.Helper()
	fixture := &lormIntegrationFixture{
		t:   t,
		ctx: ctx,
		storage: &infra.Storage{
			CoreDB: engine,
		},
	}
	fixture.project, fixture.environment = fixture.createScope()
	_, fixture.foreign = fixture.createScope()
	fixture.config = &model.Config{
		ProjectID:   fixture.project.ID,
		Key:         "settings",
		Description: "initial description",
		IsArray:     true,
		TypeJSON:    `{"type":"string"}`,
		Value:       "initial value",
	}
	_, err := NewConfigRepository(fixture.storage).Create(ctx, fixture.environment.ID, fixture.project.ID, fixture.config)
	require.NoError(t, err)
	requireInsertedMeta(t, fixture.config.Meta)
	return fixture
}

func (f *lormIntegrationFixture) createScope() (*model.Project, *model.Environment) {
	f.t.Helper()
	project := &model.Project{
		Key:         "integration-" + uuid.NewString(),
		Name:        "integration project",
		Description: "initial description",
	}
	_, err := NewProjectRepository(f.storage).Create(f.ctx, 0, project)
	require.NoError(f.t, err)
	requireInsertedMeta(f.t, project.Meta)
	environment := &model.Environment{
		ProjectID:   project.ID,
		Key:         "integration-" + uuid.NewString(),
		Name:        "integration environment",
		Description: "initial description",
	}
	_, err = NewEnvironmentRepository(f.storage).Create(f.ctx, environment, integrationAudit())
	require.NoError(f.t, err)
	requireInsertedMeta(f.t, environment.Meta)
	return project, environment
}

func integrationAudit() *model.AuditLog {
	return &model.AuditLog{
		Action:       "integration.test",
		ResourceType: "integration",
		Result:       model.AuditResultSucceeded,
	}
}

func requireInsertedMeta(t *testing.T, meta model.Meta) {
	t.Helper()
	require.Positive(t, meta.ID)
	require.EqualValues(t, 0, meta.Version)
	require.False(t, meta.CreatedAt.IsZero())
	require.False(t, meta.UpdatedAt.IsZero())
}

func verifyLormProjectEnvironment(f *lormIntegrationFixture) {
	projects := NewProjectRepository(f.storage)
	_, err := projects.LockByID(f.ctx, f.environment.ID, f.project.ID)
	require.NoError(f.t, err)
	_, err = projects.FindByID(f.ctx, f.foreign.ID, f.project.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	page, err := projects.List(f.ctx, f.environment.ID, "", PageRequest{})
	require.NoError(f.t, err)
	require.EqualValues(f.t, 1, page.Total)
	require.Len(f.t, page.Items, 1)
	_, err = projects.Update(f.ctx, f.environment.ID, f.project.ID, ProjectUpdate{
		Name:    f.project.Name,
		Version: f.project.Version,
	})
	require.NoError(f.t, err)
	project, err := projects.FindByID(f.ctx, f.environment.ID, f.project.ID)
	require.NoError(f.t, err)
	require.WithinDuration(f.t, f.project.CreatedAt, project.CreatedAt, time.Microsecond)
	require.WithinDuration(f.t, time.Now(), project.UpdatedAt, time.Second)
	require.Empty(f.t, project.Description)
	require.EqualValues(f.t, 1, project.Version)
	environments := NewEnvironmentRepository(f.storage)
	_, err = environments.Update(f.ctx, f.environment.ID, EnvironmentUpdate{
		Name:    f.environment.Name,
		Enabled: false,
		Version: f.environment.Version,
	}, integrationAudit())
	require.NoError(f.t, err)
	environment, err := environments.FindByID(f.ctx, f.environment.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, environment.Description)
	require.False(f.t, environment.Enabled)
	require.EqualValues(f.t, 1, environment.Version)
}

func verifyLormConfig(f *lormIntegrationFixture) {
	repository := NewConfigRepository(f.storage)
	_, err := repository.LockByID(f.ctx, f.environment.ID, f.config.ID)
	require.NoError(f.t, err)
	_, err = repository.FindByID(f.ctx, f.foreign.ID, f.config.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	update := ConfigUpdate{
		TypeJSON: `{"type":"string"}`,
		Version:  f.config.Version,
	}
	_, err = repository.Update(f.ctx, f.environment.ID, f.config.ID, update)
	require.NoError(f.t, err)
	_, err = repository.Update(f.ctx, f.environment.ID, f.config.ID, update)
	require.ErrorIs(f.t, err, ErrNoRowsAffected)
	config, err := repository.FindByID(f.ctx, f.environment.ID, f.config.ID)
	require.NoError(f.t, err)
	require.False(f.t, config.IsArray)
	require.Empty(f.t, config.Description)
	require.EqualValues(f.t, 1, config.Version)
	_, err = repository.UpdateValue(f.ctx, f.environment.ID, config.ID, ConfigValueUpdate{
		Version: config.Version,
	})
	require.NoError(f.t, err)
	config, err = repository.FindByID(f.ctx, f.environment.ID, config.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, config.Value)
	require.EqualValues(f.t, 2, config.Version)
	f.createStructure()
	f.createEnum()
	_, err = repository.Delete(f.ctx, f.environment.ID, config.ID)
	require.NoError(f.t, err)
	_, err = repository.FindByID(f.ctx, f.environment.ID, config.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	replacement := &model.Config{
		ProjectID: f.project.ID,
		Key:       f.config.Key,
		TypeJSON:  `{"type":"string"}`,
	}
	_, err = repository.Create(f.ctx, f.environment.ID, f.project.ID, replacement)
	require.NoError(f.t, err)
	require.NotEqual(f.t, config.ID, replacement.ID)
	require.EqualValues(f.t, 0, replacement.Version)
	structures, err := NewStructureRepository(f.storage).ListForConfig(f.ctx, f.environment.ID, config.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, structures)
	enums, err := NewConfigEnumRepository(f.storage).ListForConfig(f.ctx, f.environment.ID, config.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, enums)
	page, err := repository.List(f.ctx, f.environment.ID, ConfigFilter{
		ProjectID: &f.project.ID,
	}, PageRequest{})
	require.NoError(f.t, err)
	require.EqualValues(f.t, 1, page.Total)
}

func (f *lormIntegrationFixture) createStructure() *model.Structure {
	item := &model.Structure{
		Key:         "structure",
		Name:        "Structure",
		Description: "initial description",
		FieldsJSON:  `[{"name":"field"}]`,
	}
	_, err := NewStructureRepository(f.storage).Create(f.ctx, f.environment.ID, f.config.ID, item)
	require.NoError(f.t, err)
	requireInsertedMeta(f.t, item.Meta)
	return item
}

func verifyLormStructure(f *lormIntegrationFixture) {
	repository := NewStructureRepository(f.storage)
	item := f.createStructure()
	_, err := repository.FindByID(f.ctx, f.foreign.ID, item.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	_, err = repository.Update(f.ctx, f.environment.ID, item.ID, StructureUpdate{
		Key:        item.Key,
		FieldsJSON: "[]",
		Version:    item.Version,
	})
	require.NoError(f.t, err)
	updated, err := repository.FindByID(f.ctx, f.environment.ID, item.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, updated.Name)
	require.Empty(f.t, updated.Description)
	require.JSONEq(f.t, "[]", updated.FieldsJSON)
	require.EqualValues(f.t, 1, updated.Version)
	_, err = repository.Delete(f.ctx, f.environment.ID, item.ID)
	require.NoError(f.t, err)
	_, err = repository.FindByID(f.ctx, f.environment.ID, item.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	replacement := &model.Structure{
		Key:        item.Key,
		FieldsJSON: "[]",
	}
	_, err = repository.Create(f.ctx, f.environment.ID, f.config.ID, replacement)
	require.NoError(f.t, err)
	require.NotEqual(f.t, item.ID, replacement.ID)
	require.EqualValues(f.t, 0, replacement.Version)
	_, err = repository.Reconcile(f.ctx, f.environment.ID, f.config.ID, nil, 0)
	require.NoError(f.t, err)
	_, err = repository.Reconcile(f.ctx, f.environment.ID, f.config.ID, []*model.Structure{replacement}, 0)
	require.NoError(f.t, err)
	page, err := repository.List(f.ctx, f.environment.ID, StructureFilter{
		ProjectID: f.project.ID,
		ConfigID:  f.config.ID,
	}, PageRequest{})
	require.NoError(f.t, err)
	require.EqualValues(f.t, 1, page.Total)
	require.Len(f.t, page.Items, 1)
	require.Equal(f.t, replacement.ID, page.Items[0].ID)
	require.EqualValues(f.t, 0, page.Items[0].Version)
}

func (f *lormIntegrationFixture) createEnum() *model.ConfigEnum {
	item := &model.ConfigEnum{
		Key:         "enum",
		Name:        "Enum",
		Description: "initial description",
		ValuesJSON:  `["initial"]`,
	}
	_, err := NewConfigEnumRepository(f.storage).Create(f.ctx, f.environment.ID, f.config.ID, item)
	require.NoError(f.t, err)
	requireInsertedMeta(f.t, item.Meta)
	return item
}

func verifyLormEnum(f *lormIntegrationFixture) {
	repository := NewConfigEnumRepository(f.storage)
	item := f.createEnum()
	_, err := repository.FindByID(f.ctx, f.foreign.ID, item.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	_, err = repository.Update(f.ctx, f.environment.ID, item.ID, ConfigEnumUpdate{
		Key:        item.Key,
		ValuesJSON: "[]",
		Version:    item.Version,
	})
	require.NoError(f.t, err)
	updated, err := repository.FindByID(f.ctx, f.environment.ID, item.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, updated.Name)
	require.Empty(f.t, updated.Description)
	require.JSONEq(f.t, "[]", updated.ValuesJSON)
	require.EqualValues(f.t, 1, updated.Version)
	_, err = repository.Delete(f.ctx, f.environment.ID, item.ID)
	require.NoError(f.t, err)
	_, err = repository.FindByID(f.ctx, f.environment.ID, item.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	replacement := &model.ConfigEnum{
		Key:        item.Key,
		ValuesJSON: "[]",
	}
	_, err = repository.Create(f.ctx, f.environment.ID, f.config.ID, replacement)
	require.NoError(f.t, err)
	require.NotEqual(f.t, item.ID, replacement.ID)
	require.EqualValues(f.t, 0, replacement.Version)
	_, err = repository.Reconcile(f.ctx, f.environment.ID, f.config.ID, nil, 0)
	require.NoError(f.t, err)
	_, err = repository.Reconcile(f.ctx, f.environment.ID, f.config.ID, []*model.ConfigEnum{replacement}, 0)
	require.NoError(f.t, err)
	page, err := repository.List(f.ctx, f.environment.ID, ConfigEnumFilter{
		ProjectID: f.project.ID,
		ConfigID:  f.config.ID,
	}, PageRequest{})
	require.NoError(f.t, err)
	require.EqualValues(f.t, 1, page.Total)
	require.Len(f.t, page.Items, 1)
	require.Equal(f.t, replacement.ID, page.Items[0].ID)
	require.EqualValues(f.t, 0, page.Items[0].Version)
}

func (f *lormIntegrationFixture) createSnapshot() *model.Snapshot {
	item := &model.Snapshot{
		ConfigKey: f.config.Key,
		Content:   "{}",
		TagsJSON:  "[]",
	}
	_, err := NewSnapshotRepository(f.storage).Create(f.ctx, f.environment.ID, f.project.ID, f.config.ID, item)
	require.NoError(f.t, err)
	requireInsertedMeta(f.t, item.Meta)
	return item
}

func verifyLormSnapshot(f *lormIntegrationFixture) {
	repository := NewSnapshotRepository(f.storage)
	first := f.createSnapshot()
	second := f.createSnapshot()
	_, err := repository.LockByID(f.ctx, f.environment.ID, first.ID)
	require.NoError(f.t, err)
	_, err = repository.FindByID(f.ctx, f.foreign.ID, first.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	_, err = repository.SetCurrent(f.ctx, f.environment.ID, f.config.ID, first.ID, 0)
	require.NoError(f.t, err)
	_, err = repository.SetCurrent(f.ctx, f.environment.ID, f.config.ID, second.ID, 0)
	require.NoError(f.t, err)
	previous, err := repository.FindByID(f.ctx, f.environment.ID, first.ID)
	require.NoError(f.t, err)
	require.False(f.t, previous.IsUsing)
	require.EqualValues(f.t, 2, previous.Version)
	require.Nil(f.t, previous.PublishedAt)
	require.Nil(f.t, previous.PublishedBy)
	current, err := repository.FindCurrentForConfig(f.ctx, f.environment.ID, f.config.ID)
	require.NoError(f.t, err)
	require.Equal(f.t, second.ID, current.ID)
	require.EqualValues(f.t, 1, current.Version)
	verifyLormPublicationTimes(f, current)
	_, err = repository.Delete(f.ctx, f.environment.ID, first.ID)
	require.NoError(f.t, err)
	_, err = repository.FindByID(f.ctx, f.environment.ID, first.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	page, err := repository.List(f.ctx, f.environment.ID, SnapshotFilter{
		ProjectID: f.project.ID,
		ConfigID:  f.config.ID,
	}, PageRequest{})
	require.NoError(f.t, err)
	require.EqualValues(f.t, 1, page.Total)
}

func verifyLormPublicationTimes(f *lormIntegrationFixture, snapshot *model.Snapshot) {
	repository := NewSnapshotPublicationRepository(f.storage)
	changedAt := time.Now().In(time.FixedZone("integration UTC+8", 8*60*60))
	update := SnapshotPublicationUpdate{
		EnvironmentID:   f.environment.ID,
		ConfigID:        f.config.ID,
		SnapshotID:      snapshot.ID,
		ExpectedVersion: snapshot.Version,
		ChangedAt:       changedAt,
	}
	_, err := repository.SetReleased(f.ctx, update)
	require.NoError(f.t, err)
	released, err := NewSnapshotRepository(f.storage).FindByID(f.ctx, f.environment.ID, snapshot.ID)
	require.NoError(f.t, err)
	require.NotNil(f.t, released.PublishedAt)
	require.WithinDuration(f.t, changedAt, *released.PublishedAt, time.Microsecond)
	require.WithinDuration(f.t, changedAt, released.UpdatedAt, time.Microsecond)
	require.Equal(f.t, snapshot.Version+1, released.Version)
	update.ExpectedVersion = released.Version
	update.ChangedAt = changedAt.Add(time.Second)
	_, err = repository.SetUnreleased(f.ctx, update)
	require.NoError(f.t, err)
	unreleased, err := NewSnapshotRepository(f.storage).FindByID(f.ctx, f.environment.ID, snapshot.ID)
	require.NoError(f.t, err)
	require.Nil(f.t, unreleased.PublishedAt)
	require.Nil(f.t, unreleased.PublishedBy)
	require.WithinDuration(f.t, update.ChangedAt, unreleased.UpdatedAt, time.Microsecond)
	require.Equal(f.t, released.Version+1, unreleased.Version)
	verifyLormRuntimeVersionTime(f)
}

func verifyLormRuntimeVersionTime(f *lormIntegrationFixture) {
	fields := new(model.Config).LormCols()
	previous := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	_, err := f.storage.CoreDB.Update[*model.Config]().ID(f.config.ID).
		Set(fields.UpdatedAt(), previous).Exec(f.ctx)
	require.NoError(f.t, err)
	_, err = NewSnapshotPublicationRepository(f.storage).IncrementRuntimeVersion(f.ctx, f.environment.ID, f.config.ID)
	require.NoError(f.t, err)
	updated, err := NewConfigRepository(f.storage).FindByID(f.ctx, f.environment.ID, f.config.ID)
	require.NoError(f.t, err)
	require.Equal(f.t, f.config.RuntimeVersion+1, updated.RuntimeVersion)
	require.Equal(f.t, f.config.Version, updated.Version)
	require.WithinDuration(f.t, time.Now(), updated.UpdatedAt, time.Second)
}

func verifyLormAPIKey(f *lormIntegrationFixture) {
	repository := NewProjectAPIKeyRepository(f.storage)
	key := &model.ProjectAPIKey{
		Name:         "integration credential",
		SecretHash:   bytes.Repeat([]byte{1}, 32),
		SecretSuffix: "1234",
	}
	_, err := repository.Create(f.ctx, f.environment.ID, f.project.ID, key)
	require.NoError(f.t, err)
	require.Positive(f.t, key.ID)
	require.False(f.t, key.CreatedAt.IsZero())
	keys, err := repository.List(f.ctx, f.environment.ID, f.project.ID)
	require.NoError(f.t, err)
	require.Len(f.t, keys, 1)
	_, err = repository.LockByID(f.ctx, f.environment.ID, f.project.ID, key.ID)
	require.NoError(f.t, err)
	_, err = repository.FindByID(f.ctx, f.foreign.ID, f.project.ID, key.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	usedAt := time.Now().In(time.FixedZone("integration UTC+8", 8*60*60))
	_, err = repository.MarkUsed(f.ctx, key.SecretHash, usedAt)
	require.NoError(f.t, err)
	used, err := repository.FindByID(f.ctx, f.environment.ID, f.project.ID, key.ID)
	require.NoError(f.t, err)
	require.NotNil(f.t, used.LastUsedAt)
	require.WithinDuration(f.t, usedAt, *used.LastUsedAt, time.Microsecond)
	require.WithinDuration(f.t, usedAt, used.UpdatedAt, time.Microsecond)
	mutation := APIKeyMutation{
		EnvironmentID:     f.foreign.ID,
		ProjectID:         f.project.ID,
		KeyID:             key.ID,
		CurrentSecretHash: key.SecretHash,
	}
	replacement := &model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{
			UpdatedAt: usedAt.Add(time.Second),
		},
		SecretHash:   bytes.Repeat([]byte{2}, 32),
		SecretSuffix: "5678",
	}
	_, err = repository.Rotate(f.ctx, mutation, replacement)
	require.ErrorIs(f.t, err, ErrNoRowsAffected)
	mutation.EnvironmentID = f.environment.ID
	_, err = repository.Rotate(f.ctx, mutation, replacement)
	require.NoError(f.t, err)
	rotated, err := repository.FindByID(f.ctx, f.environment.ID, f.project.ID, key.ID)
	require.NoError(f.t, err)
	require.Equal(f.t, replacement.SecretHash, rotated.SecretHash)
	require.Nil(f.t, rotated.LastUsedAt)
	require.WithinDuration(f.t, replacement.UpdatedAt, rotated.UpdatedAt, time.Microsecond)
	mutation.CurrentSecretHash = rotated.SecretHash
	mutation.EnvironmentID = f.foreign.ID
	revokedAt := usedAt.Add(2 * time.Second)
	_, err = repository.Revoke(f.ctx, mutation, revokedAt)
	require.ErrorIs(f.t, err, ErrNoRowsAffected)
	mutation.EnvironmentID = f.environment.ID
	_, err = repository.Revoke(f.ctx, mutation, revokedAt)
	require.NoError(f.t, err)
	revoked, err := repository.FindByID(f.ctx, f.environment.ID, f.project.ID, key.ID)
	require.NoError(f.t, err)
	require.NotNil(f.t, revoked.RevokedAt)
	require.WithinDuration(f.t, revokedAt, *revoked.RevokedAt, time.Microsecond)
	require.WithinDuration(f.t, revokedAt, revoked.UpdatedAt, time.Microsecond)
}

func verifyLormInsertTimestamps(f *lormIntegrationFixture) {
	zone := time.FixedZone("integration UTC+8", 8*60*60)
	createdAt := time.Date(2024, time.June, 3, 17, 21, 19, 123000, zone)
	updatedAt := createdAt.Add(time.Hour)
	project := &model.Project{
		Meta: model.Meta{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		Key:  "timestamp-" + uuid.NewString(),
		Name: "timestamp project",
	}
	projects := NewProjectRepository(f.storage)
	_, err := projects.Create(f.ctx, 0, project)
	require.NoError(f.t, err)
	loadedProject, err := projects.FindByID(f.ctx, 0, project.ID)
	require.NoError(f.t, err)
	require.WithinDuration(f.t, createdAt, loadedProject.CreatedAt, time.Microsecond)
	require.WithinDuration(f.t, updatedAt, loadedProject.UpdatedAt, time.Microsecond)
	key := &model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		Name:         "timestamp credential",
		SecretHash:   bytes.Repeat([]byte{3}, 32),
		SecretSuffix: "9012",
	}
	keys := NewProjectAPIKeyRepository(f.storage)
	_, err = keys.Create(f.ctx, f.environment.ID, f.project.ID, key)
	require.NoError(f.t, err)
	loadedKey, err := keys.FindByID(f.ctx, f.environment.ID, f.project.ID, key.ID)
	require.NoError(f.t, err)
	require.WithinDuration(f.t, createdAt, loadedKey.CreatedAt, time.Microsecond)
	require.WithinDuration(f.t, updatedAt, loadedKey.UpdatedAt, time.Microsecond)
	record := &model.ImportRecord{
		WorkflowMeta: model.WorkflowMeta{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		SourceEnvironmentID: f.foreign.ID,
		TargetEnvironmentID: f.environment.ID,
		TargetProjectID:     f.project.ID,
		IdempotencyKey:      uuid.NewString(),
		RequestHash:         bytes.Repeat([]byte{4}, 32),
		Status:              model.ImportStatusPending,
	}
	_, created, err := NewImportRecordRepository(f.storage).Claim(f.ctx, record)
	require.NoError(f.t, err)
	require.True(f.t, created)
	loadedRecord, err := f.storage.CoreDB.Repository[*model.ImportRecord]().Get(f.ctx, record.ID)
	require.NoError(f.t, err)
	require.WithinDuration(f.t, createdAt, loadedRecord.CreatedAt, time.Microsecond)
	require.WithinDuration(f.t, updatedAt, loadedRecord.UpdatedAt, time.Microsecond)
}

func verifyLormSiblingEnvironment(f *lormIntegrationFixture) {
	sibling := &model.Environment{
		ProjectID: f.project.ID,
		Key:       "sibling",
		Name:      "Sibling",
		Enabled:   true,
	}
	_, err := NewEnvironmentRepository(f.storage).Create(f.ctx, sibling, integrationAudit())
	require.NoError(f.t, err)
	configs := NewConfigRepository(f.storage)
	_, err = configs.FindByID(f.ctx, sibling.ID, f.config.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	_, err = configs.UpdateValue(f.ctx, sibling.ID, f.config.ID, ConfigValueUpdate{})
	require.ErrorIs(f.t, err, ErrNotFound)
	_, err = configs.Delete(f.ctx, sibling.ID, f.config.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	other := &model.Config{
		ProjectID: f.project.ID,
		Key:       f.config.Key,
		Value:     "sibling value",
		TypeJSON:  `{}`,
	}
	_, err = configs.Create(f.ctx, sibling.ID, f.project.ID, other)
	require.NoError(f.t, err)
	require.NotEqual(f.t, f.config.ID, other.ID)
	snapshot := f.createSnapshot()
	_, err = NewSnapshotRepository(f.storage).FindByID(f.ctx, sibling.ID, snapshot.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	keys := NewProjectAPIKeyRepository(f.storage)
	key := &model.ProjectAPIKey{
		Name:         "isolated",
		SecretHash:   make([]byte, 32),
		SecretSuffix: "test",
	}
	_, err = keys.Create(f.ctx, f.environment.ID, f.project.ID, key)
	require.NoError(f.t, err)
	_, err = keys.FindByID(f.ctx, sibling.ID, f.project.ID, key.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	listed, err := keys.List(f.ctx, sibling.ID, f.project.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, listed)
	selected, err := keys.FindRuntimeConfig(f.ctx, sibling.ID, f.project.ID, other.Key)
	require.NoError(f.t, err)
	require.Equal(f.t, other.ID, selected.ID)
	_, err = keys.Revoke(f.ctx, APIKeyMutation{
		EnvironmentID:     sibling.ID,
		ProjectID:         f.project.ID,
		KeyID:             key.ID,
		CurrentSecretHash: key.SecretHash,
	}, time.Now())
	require.ErrorIs(f.t, err, ErrNoRowsAffected)
	_, err = keys.FindRuntimeProject(f.ctx, sibling.ID, f.project.ID)
	require.NoError(f.t, err)
	_, err = NewEnvironmentRepository(f.storage).Update(f.ctx, sibling.ID, EnvironmentUpdate{
		Name:    sibling.Name,
		Enabled: false,
		Version: sibling.Version,
	}, integrationAudit())
	require.NoError(f.t, err)
	_, err = keys.FindRuntimeProject(f.ctx, sibling.ID, f.project.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
}
