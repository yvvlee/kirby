package repository

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
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
			name: "role_admin",
			run:  verifyLormRoleAdmin,
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
	if driver == "pgx" {
		t.Run("seed_sequence_migration", func(t *testing.T) {
			verifyPostgresSeedSequenceMigration(t, dsn)
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
	require.NoError(t, NewConfigRepository(fixture.storage).Create(ctx, fixture.environment.ID, fixture.project.ID, fixture.config))
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
	require.NoError(f.t, NewProjectRepository(f.storage).Create(f.ctx, 0, project))
	requireInsertedMeta(f.t, project.Meta)
	environment := &model.Environment{
		ProjectID:   project.ID,
		Key:         "integration-" + uuid.NewString(),
		Name:        "integration environment",
		Description: "initial description",
	}
	require.NoError(f.t, NewEnvironmentRepository(f.storage).Create(f.ctx, environment, integrationAudit()))
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
	require.EqualValues(t, 1, meta.Version)
	require.False(t, meta.CreatedAt.IsZero())
	require.False(t, meta.UpdatedAt.IsZero())
	require.Nil(t, meta.DeletedAt)
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
	require.NoError(f.t, projects.Update(f.ctx, f.environment.ID, f.project.ID, ProjectUpdate{
		Name:    f.project.Name,
		Version: f.project.Version,
	}))
	project, err := projects.FindByID(f.ctx, f.environment.ID, f.project.ID)
	require.NoError(f.t, err)
	require.WithinDuration(f.t, f.project.CreatedAt, project.CreatedAt, time.Microsecond)
	require.WithinDuration(f.t, time.Now(), project.UpdatedAt, time.Second)
	require.Empty(f.t, project.Description)
	require.EqualValues(f.t, 2, project.Version)
	environments := NewEnvironmentRepository(f.storage)
	require.NoError(f.t, environments.Update(f.ctx, f.environment.ID, EnvironmentUpdate{
		Name:    f.environment.Name,
		Enabled: false,
		Version: f.environment.Version,
	}, integrationAudit()))
	environment, err := environments.FindByID(f.ctx, f.environment.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, environment.Description)
	require.False(f.t, environment.Enabled)
	require.EqualValues(f.t, 2, environment.Version)
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
	require.NoError(f.t, repository.Update(f.ctx, f.environment.ID, f.config.ID, update))
	require.ErrorIs(f.t, repository.Update(f.ctx, f.environment.ID, f.config.ID, update), ErrNoRowsAffected)
	config, err := repository.FindByID(f.ctx, f.environment.ID, f.config.ID)
	require.NoError(f.t, err)
	require.False(f.t, config.IsArray)
	require.Empty(f.t, config.Description)
	require.EqualValues(f.t, 2, config.Version)
	require.NoError(f.t, repository.UpdateValue(f.ctx, f.environment.ID, config.ID, ConfigValueUpdate{
		Version: config.Version,
	}))
	config, err = repository.FindByID(f.ctx, f.environment.ID, config.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, config.Value)
	require.EqualValues(f.t, 3, config.Version)
	f.createStructure()
	f.createEnum()
	require.NoError(f.t, repository.Delete(f.ctx, f.environment.ID, config.ID, 0))
	_, err = repository.FindByID(f.ctx, f.environment.ID, config.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	replacement := &model.Config{
		ProjectID: f.project.ID,
		Key:       f.config.Key,
		TypeJSON:  `{"type":"string"}`,
	}
	require.NoError(f.t, repository.Create(f.ctx, f.environment.ID, f.project.ID, replacement))
	require.Equal(f.t, config.ID, replacement.ID)
	require.EqualValues(f.t, 5, replacement.Version)
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
	require.NoError(f.t, NewStructureRepository(f.storage).Create(f.ctx, f.environment.ID, f.config.ID, item))
	requireInsertedMeta(f.t, item.Meta)
	return item
}

func verifyLormStructure(f *lormIntegrationFixture) {
	repository := NewStructureRepository(f.storage)
	item := f.createStructure()
	_, err := repository.FindByID(f.ctx, f.foreign.ID, item.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	require.NoError(f.t, repository.Update(f.ctx, f.environment.ID, item.ID, StructureUpdate{
		Key:        item.Key,
		FieldsJSON: "[]",
		Version:    item.Version,
	}))
	updated, err := repository.FindByID(f.ctx, f.environment.ID, item.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, updated.Name)
	require.Empty(f.t, updated.Description)
	require.JSONEq(f.t, "[]", updated.FieldsJSON)
	require.EqualValues(f.t, 2, updated.Version)
	require.NoError(f.t, repository.Delete(f.ctx, f.environment.ID, item.ID, 0))
	_, err = repository.FindByID(f.ctx, f.environment.ID, item.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	replacement := &model.Structure{
		Key:        item.Key,
		FieldsJSON: "[]",
	}
	require.NoError(f.t, repository.Create(f.ctx, f.environment.ID, f.config.ID, replacement))
	require.Equal(f.t, item.ID, replacement.ID)
	require.EqualValues(f.t, 4, replacement.Version)
	require.NoError(f.t, repository.Reconcile(f.ctx, f.environment.ID, f.config.ID, nil, 0))
	require.NoError(f.t, repository.Reconcile(f.ctx, f.environment.ID, f.config.ID, []*model.Structure{replacement}, 0))
	page, err := repository.List(f.ctx, f.environment.ID, StructureFilter{
		ProjectID: f.project.ID,
		ConfigID:  f.config.ID,
	}, PageRequest{})
	require.NoError(f.t, err)
	require.EqualValues(f.t, 1, page.Total)
	require.Len(f.t, page.Items, 1)
	require.EqualValues(f.t, 6, page.Items[0].Version)
	require.Nil(f.t, page.Items[0].DeletedAt)
}

func (f *lormIntegrationFixture) createEnum() *model.ConfigEnum {
	item := &model.ConfigEnum{
		Key:         "enum",
		Name:        "Enum",
		Description: "initial description",
		ValuesJSON:  `["initial"]`,
	}
	require.NoError(f.t, NewConfigEnumRepository(f.storage).Create(f.ctx, f.environment.ID, f.config.ID, item))
	requireInsertedMeta(f.t, item.Meta)
	return item
}

func verifyLormEnum(f *lormIntegrationFixture) {
	repository := NewConfigEnumRepository(f.storage)
	item := f.createEnum()
	_, err := repository.FindByID(f.ctx, f.foreign.ID, item.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	require.NoError(f.t, repository.Update(f.ctx, f.environment.ID, item.ID, ConfigEnumUpdate{
		Key:        item.Key,
		ValuesJSON: "[]",
		Version:    item.Version,
	}))
	updated, err := repository.FindByID(f.ctx, f.environment.ID, item.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, updated.Name)
	require.Empty(f.t, updated.Description)
	require.JSONEq(f.t, "[]", updated.ValuesJSON)
	require.EqualValues(f.t, 2, updated.Version)
	require.NoError(f.t, repository.Delete(f.ctx, f.environment.ID, item.ID, 0))
	_, err = repository.FindByID(f.ctx, f.environment.ID, item.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	replacement := &model.ConfigEnum{
		Key:        item.Key,
		ValuesJSON: "[]",
	}
	require.NoError(f.t, repository.Create(f.ctx, f.environment.ID, f.config.ID, replacement))
	require.Equal(f.t, item.ID, replacement.ID)
	require.EqualValues(f.t, 4, replacement.Version)
	require.NoError(f.t, repository.Reconcile(f.ctx, f.environment.ID, f.config.ID, nil, 0))
	require.NoError(f.t, repository.Reconcile(f.ctx, f.environment.ID, f.config.ID, []*model.ConfigEnum{replacement}, 0))
	page, err := repository.List(f.ctx, f.environment.ID, ConfigEnumFilter{
		ProjectID: f.project.ID,
		ConfigID:  f.config.ID,
	}, PageRequest{})
	require.NoError(f.t, err)
	require.EqualValues(f.t, 1, page.Total)
	require.Len(f.t, page.Items, 1)
	require.EqualValues(f.t, 6, page.Items[0].Version)
	require.Nil(f.t, page.Items[0].DeletedAt)
}

func (f *lormIntegrationFixture) createSnapshot() *model.Snapshot {
	item := &model.Snapshot{
		ConfigKey: f.config.Key,
		Content:   "{}",
		TagsJSON:  "[]",
	}
	require.NoError(f.t, NewSnapshotRepository(f.storage).Create(f.ctx, f.environment.ID, f.project.ID, f.config.ID, item))
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
	require.NoError(f.t, repository.SetCurrent(f.ctx, f.environment.ID, f.config.ID, first.ID, 0))
	require.NoError(f.t, repository.SetCurrent(f.ctx, f.environment.ID, f.config.ID, second.ID, 0))
	previous, err := repository.FindByID(f.ctx, f.environment.ID, first.ID)
	require.NoError(f.t, err)
	require.False(f.t, previous.IsUsing)
	require.EqualValues(f.t, 3, previous.Version)
	require.Nil(f.t, previous.PublishedAt)
	require.Nil(f.t, previous.PublishedBy)
	current, err := repository.FindCurrentForConfig(f.ctx, f.environment.ID, f.config.ID)
	require.NoError(f.t, err)
	require.Equal(f.t, second.ID, current.ID)
	require.EqualValues(f.t, 2, current.Version)
	verifyLormPublicationTimes(f, current)
	require.NoError(f.t, repository.Delete(f.ctx, f.environment.ID, first.ID, 0))
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
	require.NoError(f.t, repository.SetReleased(f.ctx, update))
	released, err := NewSnapshotRepository(f.storage).FindByID(f.ctx, f.environment.ID, snapshot.ID)
	require.NoError(f.t, err)
	require.NotNil(f.t, released.PublishedAt)
	require.WithinDuration(f.t, changedAt, *released.PublishedAt, time.Microsecond)
	require.WithinDuration(f.t, changedAt, released.UpdatedAt, time.Microsecond)
	require.Equal(f.t, snapshot.Version+1, released.Version)
	update.ExpectedVersion = released.Version
	update.ChangedAt = changedAt.Add(time.Second)
	require.NoError(f.t, repository.SetUnreleased(f.ctx, update))
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
	require.NoError(f.t, NewSnapshotPublicationRepository(f.storage).IncrementRuntimeVersion(f.ctx, f.environment.ID, f.config.ID))
	updated, err := NewConfigRepository(f.storage).FindByID(f.ctx, f.environment.ID, f.config.ID)
	require.NoError(f.t, err)
	require.Equal(f.t, f.config.RuntimeVersion+1, updated.RuntimeVersion)
	require.Equal(f.t, f.config.Version, updated.Version)
	require.WithinDuration(f.t, time.Now(), updated.UpdatedAt, time.Second)
}

func verifyLormAPIKey(f *lormIntegrationFixture) {
	repository := NewProjectAPIKeyRepository(f.storage)
	key := &model.ProjectAPIKey{
		PublicID:     uuid.NewString(),
		Name:         "integration credential",
		SecretHash:   bytes.Repeat([]byte{1}, 32),
		SecretSuffix: "1234",
	}
	require.NoError(f.t, repository.Create(f.ctx, f.environment.ID, f.project.ID, key))
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
	require.NoError(f.t, repository.MarkUsed(f.ctx, key.PublicID, usedAt))
	used, err := repository.FindByID(f.ctx, f.environment.ID, f.project.ID, key.ID)
	require.NoError(f.t, err)
	require.NotNil(f.t, used.LastUsedAt)
	require.WithinDuration(f.t, usedAt, *used.LastUsedAt, time.Microsecond)
	require.WithinDuration(f.t, usedAt, used.UpdatedAt, time.Microsecond)
	mutation := APIKeyMutation{
		EnvironmentID:   f.foreign.ID,
		ProjectID:       f.project.ID,
		KeyID:           key.ID,
		CurrentPublicID: key.PublicID,
	}
	replacement := &model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{
			UpdatedAt: usedAt.Add(time.Second),
		},
		PublicID:     uuid.NewString(),
		SecretHash:   bytes.Repeat([]byte{2}, 32),
		SecretSuffix: "5678",
	}
	require.ErrorIs(f.t, repository.Rotate(f.ctx, mutation, replacement), ErrNoRowsAffected)
	mutation.EnvironmentID = f.environment.ID
	require.NoError(f.t, repository.Rotate(f.ctx, mutation, replacement))
	rotated, err := repository.FindByID(f.ctx, f.environment.ID, f.project.ID, key.ID)
	require.NoError(f.t, err)
	require.Equal(f.t, replacement.PublicID, rotated.PublicID)
	require.Equal(f.t, replacement.SecretHash, rotated.SecretHash)
	require.Nil(f.t, rotated.LastUsedAt)
	require.WithinDuration(f.t, replacement.UpdatedAt, rotated.UpdatedAt, time.Microsecond)
	mutation.CurrentPublicID = rotated.PublicID
	mutation.EnvironmentID = f.foreign.ID
	revokedAt := usedAt.Add(2 * time.Second)
	require.ErrorIs(f.t, repository.Revoke(f.ctx, mutation, revokedAt), ErrNoRowsAffected)
	mutation.EnvironmentID = f.environment.ID
	require.NoError(f.t, repository.Revoke(f.ctx, mutation, revokedAt))
	revoked, err := repository.FindByID(f.ctx, f.environment.ID, f.project.ID, key.ID)
	require.NoError(f.t, err)
	require.NotNil(f.t, revoked.RevokedAt)
	require.WithinDuration(f.t, revokedAt, *revoked.RevokedAt, time.Microsecond)
	require.WithinDuration(f.t, revokedAt, revoked.UpdatedAt, time.Microsecond)
}

func verifyLormRoleAdmin(f *lormIntegrationFixture) {
	repository := NewRoleRepository(f.storage)
	role := &model.Role{
		Key:         "integration-" + uuid.NewString(),
		Name:        "integration role",
		Description: "initial description",
	}
	require.NoError(f.t, repository.Create(f.ctx, role, integrationAudit()))
	requireInsertedMeta(f.t, role.Meta)
	require.NoError(f.t, repository.Update(f.ctx, role.ID, RoleUpdate{
		Name:    role.Name,
		Version: role.Version,
	}, integrationAudit()))
	updated, err := repository.FindByID(f.ctx, role.ID)
	require.NoError(f.t, err)
	require.Empty(f.t, updated.Role.Description)
	require.EqualValues(f.t, 2, updated.Role.Version)
	permission := &model.Permission{
		Key:  "integration:" + uuid.NewString(),
		Name: "integration permission",
	}
	permission.PrepareInsert()
	_, err = f.storage.CoreDB.Repository[*model.Permission]().Insert(f.ctx, permission)
	require.NoError(f.t, err, "seeded permission IDs must advance the PostgreSQL sequence")
	_, err = repository.UpdatePermissions(f.ctx, role.ID, []int64{permission.ID}, 0, integrationAudit())
	require.NoError(f.t, err)
	assigned, err := repository.FindByID(f.ctx, role.ID)
	require.NoError(f.t, err)
	require.Len(f.t, assigned.Permissions, 1)
	require.Equal(f.t, permission.ID, assigned.Permissions[0].ID)
	_, err = repository.UpdatePermissions(f.ctx, role.ID, nil, 0, integrationAudit())
	require.NoError(f.t, err)
	require.NoError(f.t, repository.Delete(f.ctx, role.ID, 0, integrationAudit()))
	_, err = repository.FindByID(f.ctx, role.ID)
	require.ErrorIs(f.t, err, ErrNotFound)
	user, created, err := NewUserRepository(f.storage).CreateOrPromoteSystemAdmin(f.ctx, "integration-"+uuid.NewString(), "Admin", "test-hash")
	require.NoError(f.t, err)
	require.True(f.t, created)
	requireInsertedMeta(f.t, user.Meta)
	require.True(f.t, user.Enabled)
	require.True(f.t, user.IsSystemAdmin)
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
	require.NoError(f.t, projects.Create(f.ctx, 0, project))
	loadedProject, err := projects.FindByID(f.ctx, 0, project.ID)
	require.NoError(f.t, err)
	require.WithinDuration(f.t, createdAt, loadedProject.CreatedAt, time.Microsecond)
	require.WithinDuration(f.t, updatedAt, loadedProject.UpdatedAt, time.Microsecond)
	key := &model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		PublicID:     uuid.NewString(),
		Name:         "timestamp credential",
		SecretHash:   bytes.Repeat([]byte{3}, 32),
		SecretSuffix: "9012",
	}
	keys := NewProjectAPIKeyRepository(f.storage)
	require.NoError(f.t, keys.Create(f.ctx, f.environment.ID, f.project.ID, key))
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

func verifyPostgresSeedSequenceMigration(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer database.Close()
	connection, err := database.Conn(ctx)
	require.NoError(t, err)
	defer connection.Close()
	schema := "migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = connection.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	defer func() {
		_, cleanupErr := connection.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, cleanupErr)
	}()
	_, err = connection.ExecContext(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	_, err = connection.ExecContext(ctx, `
		CREATE TABLE roles (id BIGSERIAL PRIMARY KEY);
		CREATE TABLE permissions (id BIGSERIAL PRIMARY KEY);
		INSERT INTO roles (id) VALUES (1), (4);
		INSERT INTO permissions (id) VALUES (1), (21);
		SELECT setval(pg_get_serial_sequence('permissions', 'id'), 50, true);
	`)
	require.NoError(t, err)
	repair, err := os.ReadFile("../../../deploy/migrations/20261015_postgresql_seed_sequences.sql")
	require.NoError(t, err)
	_, err = connection.ExecContext(ctx, string(repair))
	require.NoError(t, err)
	_, err = connection.ExecContext(ctx, string(repair))
	require.NoError(t, err, "repair must be repeatable without rewinding either sequence")
	var roleID, permissionID int64
	require.NoError(t, connection.QueryRowContext(ctx, "INSERT INTO roles DEFAULT VALUES RETURNING id").Scan(&roleID))
	require.NoError(t, connection.QueryRowContext(ctx, "INSERT INTO permissions DEFAULT VALUES RETURNING id").Scan(&permissionID))
	require.EqualValues(t, 5, roleID)
	require.EqualValues(t, 51, permissionID)
}
