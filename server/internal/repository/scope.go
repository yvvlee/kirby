package repository

import (
	"context"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/model"
)

// Scope builders keep ownership and soft-delete predicates together. Selecting
// only the primary alias prevents joined columns from overwriting model fields.
func configScope(engine *lorm.Engine) *lorm.SelectStmt[*model.Config] {
	return engine.Query[*model.Config]().Select("c.*").From("configs AS c").
		InnerJoin("projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL").
		InnerJoin("environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL").
		Where(builder.IsNull("c.deleted_at"))
}

func structureScope(engine *lorm.Engine) *lorm.SelectStmt[*model.Structure] {
	return structureScopeIncludingDeleted(engine).Where(builder.IsNull("s.deleted_at"))
}

func structureScopeIncludingDeleted(engine *lorm.Engine) *lorm.SelectStmt[*model.Structure] {
	return engine.Query[*model.Structure]().Select("s.*").From("structures AS s").
		InnerJoin("configs AS c ON c.id = s.config_id AND c.deleted_at IS NULL").
		InnerJoin("projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL").
		InnerJoin("environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL")
}

func enumScope(engine *lorm.Engine) *lorm.SelectStmt[*model.ConfigEnum] {
	return enumScopeIncludingDeleted(engine).Where(builder.IsNull("e.deleted_at"))
}

func enumScopeIncludingDeleted(engine *lorm.Engine) *lorm.SelectStmt[*model.ConfigEnum] {
	return engine.Query[*model.ConfigEnum]().Select("e.*").From("config_enums AS e").
		InnerJoin("configs AS c ON c.id = e.config_id AND c.deleted_at IS NULL").
		InnerJoin("projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL").
		InnerJoin("environments AS env ON env.project_id = p.id AND env.deleted_at IS NULL")
}

func snapshotScope(engine *lorm.Engine) *lorm.SelectStmt[*model.Snapshot] {
	return engine.Query[*model.Snapshot]().Select("s.*").From("snapshots AS s").
		InnerJoin("configs AS c ON c.id = s.config_id AND c.project_id = s.project_id AND c.deleted_at IS NULL").
		InnerJoin("projects AS p ON p.id = s.project_id AND p.deleted_at IS NULL").
		InnerJoin("environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL").
		Where(builder.IsNull("s.deleted_at"))
}

func projectScope(engine *lorm.Engine) *lorm.SelectStmt[*model.Project] {
	return engine.Query[*model.Project]().Select("p.*").From("projects AS p").
		InnerJoin("environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL").
		Where(builder.IsNull("p.deleted_at"))
}

func apiKeyScope(engine *lorm.Engine) *lorm.SelectStmt[*model.ProjectAPIKey] {
	return engine.Query[*model.ProjectAPIKey]().Select("k.*").From("project_api_keys AS k").
		InnerJoin("projects AS p ON p.id = k.project_id AND p.deleted_at IS NULL").
		InnerJoin("environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL")
}

func lockScopedProject(ctx context.Context, engine *lorm.Engine, environmentID, projectID int64) error {
	_, found, err := projectScope(engine).Where(builder.Eq{
		"e.id": environmentID,
		"p.id": projectID,
	}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if !found {
		return Missing("project")
	}
	return nil
}

func lockScopedConfig(ctx context.Context, engine *lorm.Engine, environmentID, configID int64) error {
	_, found, err := configScope(engine).Where(builder.Eq{
		"e.id": environmentID,
		"c.id": configID,
	}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if !found {
		return Missing("config")
	}
	return nil
}

func eqIf[T any](field string, value *T) builder.Sqlizer {
	if value == nil {
		return nil
	}
	return builder.Eq{field: *value}
}

func neqIf[T any](field string, value *T) builder.Sqlizer {
	if value == nil {
		return nil
	}
	return builder.NotEq{field: *value}
}

func paginate[M any, P lorm.ModelPointer[M]](ctx context.Context, query *lorm.SelectStmt[P], page PageRequest) (PageResult[M], error) {
	page = NormalizePage(page)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return PageResult[M]{}, WrapDatabaseError(err)
	}
	result := PageResult[M]{
		Items:  make([]M, 0),
		Total:  int64(total),
		Offset: page.Offset,
		Limit:  page.Limit,
	}
	if total == 0 {
		return result, nil
	}
	rows, err := query.Limit(uint64(page.Limit)).Offset(uint64(page.Offset)).Find(ctx)
	if err != nil {
		return PageResult[M]{}, WrapDatabaseError(err)
	}
	for _, row := range rows {
		result.Items = append(result.Items, *row)
	}
	return result, nil
}
