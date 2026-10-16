package repository

import (
	"context"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type StructureFilter struct {
	ProjectID          int64
	ConfigID           int64
	IgnoreDependencyID *int64
}

type StructureUpdate struct {
	Key         string
	Name        string
	Description string
	FieldsJSON  string
	UpdatedBy   int64
	Version     int64
}

type StructureRepository interface {
	Create(ctx context.Context, environmentID, configID int64, item *model.Structure) error
	FindByID(ctx context.Context, environmentID, itemID int64) (*model.Structure, error)
	List(ctx context.Context, environmentID int64, filter StructureFilter, page PageRequest) (PageResult[model.Structure], error)
	ListForConfig(ctx context.Context, environmentID, configID int64) ([]model.Structure, error)
	Update(ctx context.Context, environmentID, itemID int64, update StructureUpdate) error
	Delete(ctx context.Context, environmentID, itemID, updatedBy int64) error
	Reconcile(ctx context.Context, environmentID, configID int64, items []*model.Structure, actorID int64) error
}

type StructureRepositoryImpl struct {
	*lorm.Repository[*model.Structure]
}

// NewStructureRepository creates StructureRepositoryImpl with its shared database engine.
//
// Parameters: storage supplies the shared database engine.
// Returns: *StructureRepositoryImpl.
// Throws: none.
func NewStructureRepository(storage *infra.Storage) *StructureRepositoryImpl {
	return &StructureRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.Structure](),
	}
}

// Create restores a deleted key while serializing writes through its parent config.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; item contains the persisted record.
// Returns: nil after insertion or restoration; IDs, version, and timestamps are copied into the supplied record. Key conflicts and database failures are returned.
// Throws: none.
func (r *StructureRepositoryImpl) Create(ctx context.Context, environmentID, configID int64, item *model.Structure) error {
	fields := new(model.Structure).LormCols()
	item.ConfigID = configID
	if err := lockScopedConfig(ctx, r.Engine, environmentID, configID); err != nil {
		return err
	}
	existing, _, err := r.Engine.Query[*model.Structure]().Where(builder.Eq{
		fields.ConfigID(): configID,
		fields.Key():      item.Key,
	}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if existing != nil {
		return r.restore(ctx, existing, item)
	}
	item.PrepareInsert()
	_, err = r.Insert(ctx, item)
	return classifyKeyWriteError("structure", err)
}

func (r *StructureRepositoryImpl) restore(ctx context.Context, existing, item *model.Structure) error {
	fields := new(model.Structure).LormCols()
	if existing.DeletedAt == nil {
		return keyConflict("structure")
	}
	now := time.Now().UTC()
	affected, err := r.Engine.Update[*model.Structure]().ID(existing.ID).SetMap(map[string]any{
		fields.Name():        item.Name,
		fields.Description(): item.Description,
		fields.FieldsJSON():  item.FieldsJSON,
		fields.CreatedBy():   item.CreatedBy,
		fields.UpdatedBy():   item.UpdatedBy,
		fields.CreatedAt():   now,
		fields.UpdatedAt():   now,
		fields.Version():     existing.Version + 1,
		fields.DeletedAt():   nil,
	}).Exec(ctx)
	if err != nil {
		return classifyKeyWriteError("structure", err)
	}
	if affected == 0 {
		return Unchanged("structure")
	}
	item.ID = existing.ID
	item.Version = existing.Version + 1
	item.CreatedAt = now
	item.UpdatedAt = now
	return nil
}

// FindByID loads a resource only when it belongs to the requested scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; itemID identifies the target child row.
// Returns: the live row, or ErrNotFound when the ID is missing, deleted, or outside the environment.
// Throws: none.
func (r *StructureRepositoryImpl) FindByID(ctx context.Context, environmentID, itemID int64) (*model.Structure, error) {
	item, found, err := structureScope(r.Engine).Where(builder.Eq{
		"e.id": environmentID,
		"s.id": itemID,
	}).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("structure")
	}
	return item, nil
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; filter restricts the returned rows; page contains pagination bounds.
// Returns: the scoped page, total row count, normalized offset and limit, or a database error.
// Throws: none.
func (r *StructureRepositoryImpl) List(ctx context.Context, environmentID int64, filter StructureFilter, page PageRequest) (PageResult[model.Structure], error) {
	condition := builder.And{
		builder.Eq{
			"e.id": environmentID,
			"p.id": filter.ProjectID,
			"c.id": filter.ConfigID,
		},
		neqIf("s.id", filter.IgnoreDependencyID),
	}
	query := structureScope(r.Engine).Where(condition).Asc("s.id")
	return paginate[model.Structure](ctx, query, page)
}

// ListForConfig locks rows in primary-key order, including deleted keys that may be restored.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: live children in ID order after locking both live and deleted keys, or a database error.
// Throws: none.
func (r *StructureRepositoryImpl) ListForConfig(ctx context.Context, environmentID, configID int64) ([]model.Structure, error) {
	rows, err := structureScopeIncludingDeleted(r.Engine).Where(builder.Eq{
		"e.id": environmentID,
		"c.id": configID,
	}).Asc("s.id").Suffix("FOR UPDATE").Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	items := make([]model.Structure, 0, len(rows))
	for _, row := range rows {
		if row.DeletedAt == nil {
			items = append(items, *row)
		}
	}
	return items, nil
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; itemID identifies the target child row; update contains the guarded changes and expected version.
// Returns: nil after one matching live row changes; ErrNoRowsAffected if the expected version is stale.
// Throws: none.
func (r *StructureRepositoryImpl) Update(ctx context.Context, environmentID, itemID int64, update StructureUpdate) error {
	fields := new(model.Structure).LormCols()
	if _, err := r.FindByID(ctx, environmentID, itemID); err != nil {
		return err
	}
	affected, err := r.Engine.Update[*model.Structure]().ID(itemID).
		Where(builder.IsNull(fields.DeletedAt())).Where(builder.Eq{fields.Version(): update.Version}).
		SetMap(map[string]any{
			fields.Key():         update.Key,
			fields.Name():        update.Name,
			fields.Description(): update.Description,
			fields.FieldsJSON():  update.FieldsJSON,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   time.Now().UTC(),
			fields.Version():     update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		return classifyKeyWriteError("structure", err)
	}
	if affected == 0 {
		return Unchanged("structure")
	}
	return nil
}

// Delete removes the resource after checking ownership and dependent state.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; itemID identifies the target child row; updatedBy identifies the actor recorded on the write.
// Returns: nil after soft deletion; ErrNotFound for an inaccessible row or ErrNoRowsAffected for a rejected write.
// Throws: none.
func (r *StructureRepositoryImpl) Delete(ctx context.Context, environmentID, itemID, updatedBy int64) error {
	fields := new(model.Structure).LormCols()
	if _, err := r.FindByID(ctx, environmentID, itemID); err != nil {
		return err
	}
	now := time.Now().UTC()
	affected, err := r.Engine.Update[*model.Structure]().ID(itemID).
		Where(builder.IsNull(fields.DeletedAt())).SetMap(map[string]any{
		fields.DeletedAt(): now,
		fields.UpdatedBy(): updatedBy,
		fields.UpdatedAt(): now,
		fields.Version():   builder.Expr("version + 1"),
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("structure")
	}
	return nil
}

// Reconcile serializes reconciliation through the parent before locking child keys in ID order.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; items contains the complete replacement set of child keys; actorID identifies the actor recorded on the write.
// Returns: nil after creating, restoring, updating, and soft-deleting child keys; returns the first write failure.
// Throws: none.
func (r *StructureRepositoryImpl) Reconcile(ctx context.Context, environmentID, configID int64, items []*model.Structure, actorID int64) error {
	fields := new(model.Structure).LormCols()
	if err := lockScopedConfig(ctx, r.Engine, environmentID, configID); err != nil {
		return err
	}
	existing, err := r.Engine.Query[*model.Structure]().Where(builder.Eq{fields.ConfigID(): configID}).
		Asc("id").Suffix("FOR UPDATE").Find(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	byKey := make(map[string]*model.Structure, len(existing))
	for _, current := range existing {
		byKey[current.Key] = current
	}
	desired := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.ConfigID = configID
		item.UpdatedBy = actorID
		desired[item.Key] = struct{}{}
		if err := r.reconcileItem(ctx, byKey[item.Key], item); err != nil {
			return err
		}
	}
	return r.removeObsolete(ctx, existing, desired, actorID)
}

func (r *StructureRepositoryImpl) reconcileItem(ctx context.Context, current, item *model.Structure) error {
	fields := new(model.Structure).LormCols()
	if current == nil {
		item.ID = 0
		item.CreatedBy = item.UpdatedBy
		item.PrepareInsert()
		_, err := r.Insert(ctx, item)
		return classifyKeyWriteError("structure", err)
	}
	affected, err := r.Engine.Update[*model.Structure]().ID(current.ID).
		Where(builder.Eq{fields.ConfigID(): item.ConfigID}).SetMap(map[string]any{
		fields.Name():        item.Name,
		fields.Description(): item.Description,
		fields.FieldsJSON():  item.FieldsJSON,
		fields.DeletedAt():   nil,
		fields.UpdatedBy():   item.UpdatedBy,
		fields.UpdatedAt():   time.Now().UTC(),
		fields.Version():     current.Version + 1,
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("structure")
	}
	item.ID = current.ID
	item.Version = current.Version + 1
	return nil
}

func (r *StructureRepositoryImpl) removeObsolete(ctx context.Context, existing []*model.Structure, desired map[string]struct{}, actorID int64) error {
	for _, current := range existing {
		if _, keep := desired[current.Key]; keep || current.DeletedAt != nil {
			continue
		}
		if err := r.removeObsoleteItem(ctx, current, actorID); err != nil {
			return err
		}
	}
	return nil
}

func (r *StructureRepositoryImpl) removeObsoleteItem(ctx context.Context, current *model.Structure, actorID int64) error {
	fields := new(model.Structure).LormCols()
	now := time.Now().UTC()
	affected, err := r.Engine.Update[*model.Structure]().ID(current.ID).
		Where(builder.IsNull(fields.DeletedAt())).Where(builder.Eq{fields.ConfigID(): current.ConfigID}).
		SetMap(map[string]any{
			fields.DeletedAt(): now,
			fields.UpdatedBy(): actorID,
			fields.UpdatedAt(): now,
			fields.Version():   current.Version + 1,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("structure")
	}
	return nil
}

var _ StructureRepository = (*StructureRepositoryImpl)(nil)
