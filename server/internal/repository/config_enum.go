package repository

import (
	"context"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type ConfigEnumFilter struct {
	ProjectID int64
	ConfigID  int64
}

type ConfigEnumUpdate struct {
	Key         string
	Name        string
	Description string
	ValuesJSON  string
	UpdatedBy   int64
	Version     int64
}

type ConfigEnumRepository interface {
	Create(ctx context.Context, environmentID, configID int64, item *model.ConfigEnum) (rowsAffected int64, err error)
	FindByID(ctx context.Context, environmentID, itemID int64) (*model.ConfigEnum, error)
	List(ctx context.Context, environmentID int64, filter ConfigEnumFilter, page PageRequest) (PageResult[model.ConfigEnum], error)
	ListForConfig(ctx context.Context, environmentID, configID int64) ([]model.ConfigEnum, error)
	Update(ctx context.Context, environmentID, itemID int64, update ConfigEnumUpdate) (rowsAffected int64, err error)
	Delete(ctx context.Context, environmentID, itemID int64) (rowsAffected int64, err error)
	Reconcile(ctx context.Context, environmentID, configID int64, items []*model.ConfigEnum, actorID int64) (rowsAffected int64, err error)
}

type ConfigEnumRepositoryImpl struct {
	*lorm.Repository[*model.ConfigEnum]
}

// NewConfigEnumRepository creates ConfigEnumRepositoryImpl with its shared database engine.
//
// Parameters: storage supplies the shared database engine.
// Returns: *ConfigEnumRepositoryImpl.
// Throws: none.
func NewConfigEnumRepository(storage *infra.Storage) *ConfigEnumRepositoryImpl {
	return &ConfigEnumRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.ConfigEnum](),
	}
}

// Create inserts a new key while serializing writes through its parent config.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; item contains the persisted record.
// Returns: rowsAffected from the insert; IDs, version, and timestamps are copied into the supplied record. Key conflicts and database failures are returned.
// Throws: none.
func (r *ConfigEnumRepositoryImpl) Create(ctx context.Context, environmentID, configID int64, item *model.ConfigEnum) (rowsAffected int64, err error) {
	item.ConfigID = configID
	if err = lockScopedConfig(ctx, r.Engine, environmentID, configID); err != nil {
		return
	}
	rowsAffected, err = r.Insert(ctx, item)
	err = classifyKeyWriteError("config enum", err)
	return
}

// FindByID loads a resource only when it belongs to the requested scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; itemID identifies the target child row.
// Returns: the live row, or ErrNotFound when the ID is missing, deleted, or outside the environment.
// Throws: none.
func (r *ConfigEnumRepositoryImpl) FindByID(ctx context.Context, environmentID, itemID int64) (*model.ConfigEnum, error) {
	item, found, err := enumScope(r.Engine).Where(builder.Eq{
		"env.id": environmentID,
		"e.id":   itemID,
	}).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("config enum")
	}
	return item, nil
}

// List returns the resources visible in the selected scope.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; filter restricts the returned rows; page contains pagination bounds.
// Returns: the scoped page, total row count, normalized offset and limit, or a database error.
// Throws: none.
func (r *ConfigEnumRepositoryImpl) List(ctx context.Context, environmentID int64, filter ConfigEnumFilter, page PageRequest) (PageResult[model.ConfigEnum], error) {
	condition := builder.And{
		builder.Eq{
			"env.id": environmentID,
			"p.id":   filter.ProjectID,
			"c.id":   filter.ConfigID,
		},
	}
	query := enumScope(r.Engine).Where(condition).Asc("e.id")
	return paginate[model.ConfigEnum](ctx, query, page)
}

// ListForConfig locks rows in primary-key order.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration.
// Returns: live children in ID order after locking, or a database error.
// Throws: none.
func (r *ConfigEnumRepositoryImpl) ListForConfig(ctx context.Context, environmentID, configID int64) ([]model.ConfigEnum, error) {
	rows, err := enumScope(r.Engine).Where(builder.Eq{
		"env.id": environmentID,
		"c.id":   configID,
	}).Asc("e.id").Suffix("FOR UPDATE").Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(rows), nil
}

// Update applies changes using the expected version to reject stale writes.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; itemID identifies the target child row; update contains the guarded changes and expected version.
// Returns: rowsAffected after one matching live row changes; ErrNoRowsAffected if the expected version is stale.
// Throws: none.
func (r *ConfigEnumRepositoryImpl) Update(ctx context.Context, environmentID, itemID int64, update ConfigEnumUpdate) (rowsAffected int64, err error) {
	fields := new(model.ConfigEnum).LormCols()
	if _, err = r.FindByID(ctx, environmentID, itemID); err != nil {
		return
	}
	rowsAffected, err = r.Engine.Update[*model.ConfigEnum]().ID(itemID).
		Where(builder.Eq{fields.Version(): update.Version}).
		SetMap(map[string]any{
			fields.Key():         update.Key,
			fields.Name():        update.Name,
			fields.Description(): update.Description,
			fields.ValuesJSON():  update.ValuesJSON,
			fields.UpdatedBy():   update.UpdatedBy,
			fields.UpdatedAt():   time.Now(),
			fields.Version():     update.Version + 1,
		}).Exec(ctx)
	if err != nil {
		err = classifyKeyWriteError("config enum", err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("config enum")
		return
	}
	return
}

// Delete 物理删除资源，删除前检查归属与依赖状态。
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; itemID identifies the target child row.
// Returns: rowsAffected after the physical delete; ErrNotFound for an inaccessible row or ErrNoRowsAffected for a rejected write.
// Throws: none.
func (r *ConfigEnumRepositoryImpl) Delete(ctx context.Context, environmentID, itemID int64) (rowsAffected int64, err error) {
	if _, err = r.FindByID(ctx, environmentID, itemID); err != nil {
		return
	}
	rowsAffected, err = r.Engine.Delete[*model.ConfigEnum]().ID(itemID).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("config enum")
		return
	}
	return
}

// Reconcile serializes reconciliation through the parent before locking child keys in ID order.
//
// Parameters: ctx carries cancellation and transaction state; environmentID selects the authorized environment; configID selects the configuration; items contains the complete replacement set of child keys; actorID identifies the actor recorded on new rows.
// Returns: rowsAffected of the last write, counting the final inserted or updated item or the last removed obsolete key; returns the first write failure.
// Throws: none.
func (r *ConfigEnumRepositoryImpl) Reconcile(ctx context.Context, environmentID, configID int64, items []*model.ConfigEnum, actorID int64) (rowsAffected int64, err error) {
	fields := new(model.ConfigEnum).LormCols()
	if err = lockScopedConfig(ctx, r.Engine, environmentID, configID); err != nil {
		return
	}
	existing, err := r.Engine.Query[*model.ConfigEnum]().Where(builder.Eq{fields.ConfigID(): configID}).
		Asc("id").Suffix("FOR UPDATE").Find(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	byKey := make(map[string]*model.ConfigEnum, len(existing))
	for _, current := range existing {
		byKey[current.Key] = current
	}
	desired := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.ConfigID = configID
		item.UpdatedBy = actorID
		desired[item.Key] = struct{}{}
		if rowsAffected, err = r.reconcileItem(ctx, byKey[item.Key], item); err != nil {
			return
		}
	}
	for _, current := range existing {
		if _, keep := desired[current.Key]; keep {
			continue
		}
		if rowsAffected, err = r.removeObsoleteItem(ctx, current); err != nil {
			return
		}
	}
	return
}

func (r *ConfigEnumRepositoryImpl) reconcileItem(ctx context.Context, current, item *model.ConfigEnum) (rowsAffected int64, err error) {
	fields := new(model.ConfigEnum).LormCols()
	if current == nil {
		item.ID = 0
		item.CreatedBy = item.UpdatedBy
		rowsAffected, err = r.Insert(ctx, item)
		err = classifyKeyWriteError("config enum", err)
		return
	}
	rowsAffected, err = r.Engine.Update[*model.ConfigEnum]().ID(current.ID).
		Where(builder.Eq{fields.ConfigID(): item.ConfigID}).SetMap(map[string]any{
		fields.Name():        item.Name,
		fields.Description(): item.Description,
		fields.ValuesJSON():  item.ValuesJSON,
		fields.UpdatedBy():   item.UpdatedBy,
		fields.UpdatedAt():   time.Now(),
		fields.Version():     current.Version + 1,
	}).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("config enum")
		return
	}
	item.ID = current.ID
	item.Version = current.Version + 1
	return
}

func (r *ConfigEnumRepositoryImpl) removeObsoleteItem(ctx context.Context, current *model.ConfigEnum) (rowsAffected int64, err error) {
	rowsAffected, err = r.Engine.Delete[*model.ConfigEnum]().ID(current.ID).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("config enum")
		return
	}
	return
}

var _ ConfigEnumRepository = (*ConfigEnumRepositoryImpl)(nil)
