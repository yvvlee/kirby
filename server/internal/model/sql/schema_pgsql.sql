-- Kirby schema for PostgreSQL.
--
-- The service runs this embedded schema for an empty database and generates
-- installation secrets in the seed transaction. This file is also a schema
-- reference. IF NOT EXISTS is omitted so incomplete initialization fails early.


CREATE TABLE "application_secrets" (
  "id" BIGINT NOT NULL,
  "jwt_key_id" VARCHAR(32) NOT NULL,
  "jwt_signing_key" VARCHAR(64) NOT NULL,
  "api_key_pepper" VARCHAR(64) NOT NULL,
  "admin_initialized" BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY ("id"),
  CONSTRAINT "ck_application_secrets_singleton" CHECK ("id" = 1)
);

CREATE TABLE "environments" (
  "id" BIGSERIAL NOT NULL,
  "project_id" BIGINT NOT NULL,
  "key" VARCHAR(64) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "enabled" BOOLEAN NOT NULL DEFAULT TRUE,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  "deleted_at" TIMESTAMP(6) NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_environments_project_key" UNIQUE ("project_id", "key")
);

CREATE TABLE "users" (
  "id" BIGSERIAL NOT NULL,
  "username" VARCHAR(128) NOT NULL,
  "display_name" VARCHAR(128) NOT NULL,
  "password_hash" VARCHAR(255) NOT NULL,
  "enabled" BOOLEAN NOT NULL DEFAULT TRUE,
  "is_system_admin" BOOLEAN NOT NULL DEFAULT FALSE,
  "must_change_password" BOOLEAN NOT NULL DEFAULT FALSE,
  "last_login_at" TIMESTAMP(6) NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  "deleted_at" TIMESTAMP(6) NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_users_username" UNIQUE ("username")
);

CREATE TABLE "roles" (
  "id" BIGSERIAL NOT NULL,
  "key" VARCHAR(64) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "builtin" BOOLEAN NOT NULL DEFAULT FALSE,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  "deleted_at" TIMESTAMP(6) NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_roles_key" UNIQUE ("key")
);

CREATE TABLE "permissions" (
  "id" BIGSERIAL NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "name" VARCHAR(128) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_permissions_key" UNIQUE ("key")
);

CREATE TABLE "user_environment_roles" (
  "id" BIGSERIAL NOT NULL,
  "user_id" BIGINT NOT NULL,
  "environment_id" BIGINT NOT NULL,
  "role_id" BIGINT NOT NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_user_environment_roles_assignment" UNIQUE ("user_id", "environment_id", "role_id")
);

CREATE TABLE "role_permissions" (
  "id" BIGSERIAL NOT NULL,
  "role_id" BIGINT NOT NULL,
  "permission_id" BIGINT NOT NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_role_permissions_assignment" UNIQUE ("role_id", "permission_id")
);

CREATE TABLE "refresh_tokens" (
  "id" BIGSERIAL NOT NULL,
  "user_id" BIGINT NOT NULL,
  "session_id" VARCHAR(64) NOT NULL,
  "token_hash" BYTEA NOT NULL,
  "expires_at" TIMESTAMP(6) NOT NULL,
  "last_used_at" TIMESTAMP(6) NULL,
  "revoked_at" TIMESTAMP(6) NULL,
  "replaced_by_id" BIGINT NULL,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_refresh_tokens_hash" UNIQUE ("token_hash")
);

CREATE TABLE "projects" (
  "id" BIGSERIAL NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  "deleted_at" TIMESTAMP(6) NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_projects_key" UNIQUE ("key")
);

CREATE TABLE "project_api_keys" (
  "id" BIGSERIAL NOT NULL,
  "project_id" BIGINT NOT NULL,
  "public_id" VARCHAR(64) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "secret_hash" BYTEA NOT NULL,
  "secret_suffix" CHAR(4) NOT NULL,
  "created_by" BIGINT NOT NULL,
  "last_used_at" TIMESTAMP(6) NULL,
  "revoked_at" TIMESTAMP(6) NULL,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_project_api_keys_public_id" UNIQUE ("public_id")
);

CREATE TABLE "configs" (
  "id" BIGSERIAL NOT NULL,
  "project_id" BIGINT NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "is_array" BOOLEAN NOT NULL DEFAULT FALSE,
  "type_json" JSON NOT NULL,
  "value" TEXT NOT NULL,
  "runtime_version" BIGINT NOT NULL DEFAULT 0,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  "deleted_at" TIMESTAMP(6) NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_configs_project_key" UNIQUE ("project_id", "key"),
  CONSTRAINT "ck_configs_runtime_version" CHECK ("runtime_version" >= 0)
);

CREATE TABLE "structures" (
  "id" BIGSERIAL NOT NULL,
  "config_id" BIGINT NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "fields_json" JSON NOT NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  "deleted_at" TIMESTAMP(6) NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_structures_config_key" UNIQUE ("config_id", "key")
);

CREATE TABLE "config_enums" (
  "id" BIGSERIAL NOT NULL,
  "config_id" BIGINT NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "values_json" JSON NOT NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  "deleted_at" TIMESTAMP(6) NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_config_enums_config_key" UNIQUE ("config_id", "key")
);

CREATE TABLE "snapshots" (
  "id" BIGSERIAL NOT NULL,
  "project_id" BIGINT NOT NULL,
  "config_id" BIGINT NOT NULL,
  "config_key" VARCHAR(128) NOT NULL,
  "description" VARCHAR(255) NOT NULL,
  "content" TEXT NOT NULL,
  "status" SMALLINT NOT NULL DEFAULT 1,
  "tags_json" JSON NOT NULL,
  "is_using" BOOLEAN NOT NULL DEFAULT FALSE,
  "published_at" TIMESTAMP(6) NULL,
  "published_by" BIGINT NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  "deleted_at" TIMESTAMP(6) NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "ck_snapshots_status" CHECK ("status" IN (1, 3))
);

CREATE TABLE "import_records" (
  "id" BIGSERIAL NOT NULL,
  "user_id" BIGINT NOT NULL,
  "source_environment_id" BIGINT NOT NULL,
  "target_environment_id" BIGINT NOT NULL,
  "source_snapshot_id" BIGINT NOT NULL,
  "target_project_id" BIGINT NOT NULL,
  "target_snapshot_id" BIGINT NULL,
  "idempotency_key" VARCHAR(128) NOT NULL,
  "request_hash" BYTEA NOT NULL,
  "status" VARCHAR(16) NOT NULL,
  "result_json" JSON NULL,
  "error_message" VARCHAR(1024) NOT NULL DEFAULT '',
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_import_records_idempotency" UNIQUE ("user_id", "target_environment_id", "idempotency_key"),
  CONSTRAINT "ck_import_records_status" CHECK ("status" IN ('pending', 'succeeded', 'failed'))
);

CREATE TABLE "audit_logs" (
  "id" BIGSERIAL NOT NULL,
  "actor_user_id" BIGINT NULL,
  "environment_id" BIGINT NULL,
  "action" VARCHAR(128) NOT NULL,
  "resource_type" VARCHAR(64) NOT NULL,
  "resource_id" VARCHAR(128) NOT NULL DEFAULT '',
  "result" VARCHAR(16) NOT NULL,
  "request_id" VARCHAR(128) NOT NULL,
  "details_json" JSON NULL,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  CONSTRAINT "ck_audit_logs_result" CHECK ("result" IN ('succeeded', 'failed'))
);

START TRANSACTION;


INSERT INTO "permissions" ("id", "key", "name", "description") VALUES
  (1,  'project:read',               '读取项目',       '查看项目'),
  (2,  'project:write',              '管理项目',       '创建和修改项目'),
  (3,  'project:api_key:read',       '读取项目密钥',   '查看项目 API Key 元数据'),
  (4,  'project:api_key:manage',     '管理项目密钥',   '创建、轮换和撤销项目 API Key'),
  (5,  'config:read',                '读取配置',       '查看配置内容'),
  (6,  'config:write',               '管理配置',       '创建、修改和删除配置'),
  (7,  'structure:read',             '读取结构',       '查看配置结构'),
  (8,  'structure:write',            '管理结构',       '创建、修改和删除配置结构'),
  (9,  'enum:read',                  '读取枚举',       '查看配置枚举'),
  (10, 'enum:write',                 '管理枚举',       '创建、修改和删除配置枚举'),
  (11, 'snapshot:read',              '读取快照',       '查看配置快照'),
  (12, 'snapshot:write',             '管理快照',       '创建和删除未发布快照'),
  (13, 'snapshot:publish',           '发布快照',       '发布和下线配置快照'),
  (14, 'snapshot:export',            '导出快照',       '导出有权读取的快照'),
  (15, 'snapshot:import',            '导入快照',       '向有权写入的环境导入快照'),
  (16, 'asset:write',                '上传资源',       '向项目对象存储上传资源'),
  (17, 'environment:member:manage',  '管理环境成员',   '调整当前环境的成员角色'),
  (18, 'system:user:manage',         '管理系统用户',   '创建、修改和停用系统用户'),
  (19, 'system:role:manage',         '管理系统角色',   '创建、修改角色和权限'),
  (20, 'system:environment:manage',  '管理系统环境',   '创建和修改环境'),
  (21, 'system:project:manage',      '管理系统项目',   '创建和修改项目');

INSERT INTO "roles" ("id", "key", "name", "description", "builtin", "created_by", "updated_by") VALUES
  (1, 'viewer',    '只读成员', '可以读取和导出环境内的配置', TRUE, 0, 0),
  (2, 'editor',    '编辑成员', '可以编辑环境内的配置和导入快照', TRUE, 0, 0),
  (3, 'publisher', '发布成员', '包含编辑权限，并可以发布快照', TRUE, 0, 0),
  (4, 'admin',     '环境管理员', '包含全部环境级权限，并可以管理环境成员和项目密钥', TRUE, 0, 0);

-- Explicit seed IDs do not advance BIGSERIAL sequences in PostgreSQL.
-- Start subsequent application inserts after the seeded catalog records.
SELECT setval(pg_get_serial_sequence('permissions', 'id'), (SELECT MAX(id) FROM permissions));
SELECT setval(pg_get_serial_sequence('roles', 'id'), (SELECT MAX(id) FROM roles));

-- viewer
INSERT INTO "role_permissions" ("role_id", "permission_id", "created_by") VALUES
  (1, 1, 0), (1, 3, 0), (1, 5, 0), (1, 7, 0), (1, 9, 0), (1, 11, 0), (1, 14, 0);
-- editor = viewer + environment content mutation, import, and asset upload
INSERT INTO "role_permissions" ("role_id", "permission_id", "created_by") VALUES
  (2, 1, 0), (2, 3, 0), (2, 5, 0), (2, 7, 0), (2, 9, 0), (2, 11, 0), (2, 14, 0),
  (2, 2, 0), (2, 6, 0), (2, 8, 0), (2, 10, 0), (2, 12, 0), (2, 15, 0), (2, 16, 0);
-- publisher = editor + publish
INSERT INTO "role_permissions" ("role_id", "permission_id", "created_by") VALUES
  (3, 1, 0), (3, 3, 0), (3, 5, 0), (3, 7, 0), (3, 9, 0), (3, 11, 0), (3, 14, 0),
  (3, 2, 0), (3, 6, 0), (3, 8, 0), (3, 10, 0), (3, 12, 0), (3, 15, 0), (3, 16, 0),
  (3, 13, 0);
-- admin = publisher + project API Key management + environment membership
INSERT INTO "role_permissions" ("role_id", "permission_id", "created_by") VALUES
  (4, 1, 0), (4, 3, 0), (4, 5, 0), (4, 7, 0), (4, 9, 0), (4, 11, 0), (4, 14, 0),
  (4, 2, 0), (4, 6, 0), (4, 8, 0), (4, 10, 0), (4, 12, 0), (4, 15, 0), (4, 16, 0),
  (4, 13, 0), (4, 4, 0), (4, 17, 0);

-- System permissions (18-21) are deliberately not assigned to an environment
-- role. They require users.is_system_admin and are checked separately.

COMMIT;
