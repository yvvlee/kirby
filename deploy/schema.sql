-- Kirby schema for MySQL 8.0.
--
-- Run this file manually against an empty database exactly once. The Kirby
-- service never creates or alters tables. This file intentionally does not use
-- IF NOT EXISTS so an accidental second execution fails immediately.

SET NAMES utf8mb4 COLLATE utf8mb4_0900_ai_ci;
SET time_zone = '+00:00';

CREATE TABLE `environments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `key` VARCHAR(64) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `enabled` BOOLEAN NOT NULL DEFAULT TRUE,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `deleted_at` DATETIME(6) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_environments_key` (`key`),
  KEY `ix_environments_enabled_deleted` (`enabled`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(128) NOT NULL,
  `display_name` VARCHAR(128) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `enabled` BOOLEAN NOT NULL DEFAULT TRUE,
  `is_system_admin` BOOLEAN NOT NULL DEFAULT FALSE,
  `last_login_at` DATETIME(6) NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `deleted_at` DATETIME(6) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_users_username` (`username`),
  KEY `ix_users_enabled_deleted` (`enabled`, `deleted_at`),
  KEY `ix_users_system_admin` (`is_system_admin`, `enabled`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `roles` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `key` VARCHAR(64) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `builtin` BOOLEAN NOT NULL DEFAULT FALSE,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `deleted_at` DATETIME(6) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_roles_key` (`key`),
  KEY `ix_roles_builtin_deleted` (`builtin`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `permissions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `key` VARCHAR(128) NOT NULL,
  `name` VARCHAR(128) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_permissions_key` (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `user_environment_roles` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `environment_id` BIGINT UNSIGNED NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_user_environment_roles_assignment` (`user_id`, `environment_id`, `role_id`),
  KEY `ix_user_environment_roles_environment_user` (`environment_id`, `user_id`),
  KEY `ix_user_environment_roles_role` (`role_id`),
  CONSTRAINT `fk_user_environment_roles_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_user_environment_roles_environment` FOREIGN KEY (`environment_id`) REFERENCES `environments` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_user_environment_roles_role` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `role_permissions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `role_id` BIGINT UNSIGNED NOT NULL,
  `permission_id` BIGINT UNSIGNED NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_role_permissions_assignment` (`role_id`, `permission_id`),
  KEY `ix_role_permissions_permission` (`permission_id`),
  CONSTRAINT `fk_role_permissions_role` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_role_permissions_permission` FOREIGN KEY (`permission_id`) REFERENCES `permissions` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `refresh_tokens` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `session_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `token_hash` BINARY(32) NOT NULL,
  `expires_at` DATETIME(6) NOT NULL,
  `last_used_at` DATETIME(6) NULL,
  `revoked_at` DATETIME(6) NULL,
  `replaced_by_id` BIGINT UNSIGNED NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_refresh_tokens_hash` (`token_hash`),
  KEY `ix_refresh_tokens_user_session` (`user_id`, `session_id`),
  KEY `ix_refresh_tokens_active_expiry` (`revoked_at`, `expires_at`),
  KEY `ix_refresh_tokens_replaced_by` (`replaced_by_id`),
  CONSTRAINT `fk_refresh_tokens_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_refresh_tokens_replaced_by` FOREIGN KEY (`replaced_by_id`) REFERENCES `refresh_tokens` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `projects` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `environment_id` BIGINT UNSIGNED NOT NULL,
  `key` VARCHAR(128) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `deleted_at` DATETIME(6) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_projects_environment_key` (`environment_id`, `key`),
  KEY `ix_projects_environment_deleted` (`environment_id`, `deleted_at`),
  CONSTRAINT `fk_projects_environment` FOREIGN KEY (`environment_id`) REFERENCES `environments` (`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `project_api_keys` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `public_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `secret_hash` BINARY(32) NOT NULL,
  `secret_suffix` CHAR(4) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL,
  `last_used_at` DATETIME(6) NULL,
  `revoked_at` DATETIME(6) NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_project_api_keys_public_id` (`public_id`),
  KEY `ix_project_api_keys_project_active` (`project_id`, `revoked_at`),
  CONSTRAINT `fk_project_api_keys_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `configs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `key` VARCHAR(128) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `is_array` BOOLEAN NOT NULL DEFAULT FALSE,
  `type_json` JSON NOT NULL,
  `value` MEDIUMTEXT NOT NULL,
  `runtime_version` BIGINT NOT NULL DEFAULT 0,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `deleted_at` DATETIME(6) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_configs_project_key` (`project_id`, `key`),
  KEY `ix_configs_project_deleted` (`project_id`, `deleted_at`),
  KEY `ix_configs_runtime_version` (`project_id`, `key`, `runtime_version`),
  CONSTRAINT `fk_configs_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `ck_configs_runtime_version` CHECK (`runtime_version` >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `structures` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `config_id` BIGINT UNSIGNED NOT NULL,
  `key` VARCHAR(128) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `fields_json` JSON NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `deleted_at` DATETIME(6) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_structures_config_key` (`config_id`, `key`),
  KEY `ix_structures_config_deleted` (`config_id`, `deleted_at`),
  CONSTRAINT `fk_structures_config` FOREIGN KEY (`config_id`) REFERENCES `configs` (`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `config_enums` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `config_id` BIGINT UNSIGNED NOT NULL,
  `key` VARCHAR(128) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `values_json` JSON NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `deleted_at` DATETIME(6) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_config_enums_config_key` (`config_id`, `key`),
  KEY `ix_config_enums_config_deleted` (`config_id`, `deleted_at`),
  CONSTRAINT `fk_config_enums_config` FOREIGN KEY (`config_id`) REFERENCES `configs` (`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `snapshots` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `config_id` BIGINT UNSIGNED NOT NULL,
  `config_key` VARCHAR(128) NOT NULL,
  `description` VARCHAR(255) NOT NULL,
  `content` MEDIUMTEXT NOT NULL,
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1,
  `tags_json` JSON NOT NULL,
  `is_using` BOOLEAN NOT NULL DEFAULT FALSE,
  `published_at` DATETIME(6) NULL,
  `published_by` BIGINT UNSIGNED NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `deleted_at` DATETIME(6) NULL,
  PRIMARY KEY (`id`),
  KEY `ix_snapshots_config_created` (`config_id`, `created_at`),
  KEY `ix_snapshots_project_config_status` (`project_id`, `config_id`, `status`, `deleted_at`),
  KEY `ix_snapshots_runtime_lookup` (`config_id`, `status`, `is_using`, `deleted_at`),
  CONSTRAINT `fk_snapshots_project` FOREIGN KEY (`project_id`) REFERENCES `projects` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `fk_snapshots_config` FOREIGN KEY (`config_id`) REFERENCES `configs` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `ck_snapshots_status` CHECK (`status` IN (1, 3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `import_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `source_environment_id` BIGINT UNSIGNED NOT NULL,
  `target_environment_id` BIGINT UNSIGNED NOT NULL,
  `source_snapshot_id` BIGINT UNSIGNED NOT NULL,
  `target_project_id` BIGINT UNSIGNED NOT NULL,
  `target_snapshot_id` BIGINT UNSIGNED NULL,
  `idempotency_key` VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `request_hash` BINARY(32) NOT NULL,
  `status` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `result_json` JSON NULL,
  `error_message` VARCHAR(1024) NOT NULL DEFAULT '',
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_import_records_idempotency` (`user_id`, `target_environment_id`, `idempotency_key`),
  KEY `ix_import_records_source` (`source_environment_id`, `source_snapshot_id`),
  KEY `ix_import_records_target` (`target_environment_id`, `target_project_id`, `created_at`),
  KEY `ix_import_records_target_snapshot` (`target_snapshot_id`),
  CONSTRAINT `fk_import_records_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `fk_import_records_source_environment` FOREIGN KEY (`source_environment_id`) REFERENCES `environments` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `fk_import_records_target_environment` FOREIGN KEY (`target_environment_id`) REFERENCES `environments` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `fk_import_records_source_snapshot` FOREIGN KEY (`source_snapshot_id`) REFERENCES `snapshots` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `fk_import_records_target_project` FOREIGN KEY (`target_project_id`) REFERENCES `projects` (`id`) ON DELETE RESTRICT,
  CONSTRAINT `fk_import_records_target_snapshot` FOREIGN KEY (`target_snapshot_id`) REFERENCES `snapshots` (`id`) ON DELETE SET NULL,
  CONSTRAINT `ck_import_records_status` CHECK (`status` IN ('pending', 'succeeded', 'failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `audit_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `actor_user_id` BIGINT UNSIGNED NULL,
  `environment_id` BIGINT UNSIGNED NULL,
  `action` VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `resource_type` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `resource_id` VARCHAR(128) NOT NULL DEFAULT '',
  `result` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `request_id` VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `details_json` JSON NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `ix_audit_logs_environment_created` (`environment_id`, `created_at`),
  KEY `ix_audit_logs_actor_created` (`actor_user_id`, `created_at`),
  KEY `ix_audit_logs_request` (`request_id`),
  KEY `ix_audit_logs_resource` (`resource_type`, `resource_id`, `created_at`),
  CONSTRAINT `ck_audit_logs_result` CHECK (`result` IN ('succeeded', 'failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

START TRANSACTION;

INSERT INTO `permissions` (`id`, `key`, `name`, `description`) VALUES
  (1,  'project:read',               '读取项目',       '查看环境内的项目'),
  (2,  'project:write',              '管理项目',       '创建和修改环境内的项目'),
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
  (20, 'system:environment:manage',  '管理系统环境',   '创建和修改环境');

INSERT INTO `roles` (`id`, `key`, `name`, `description`, `builtin`, `created_by`, `updated_by`) VALUES
  (1, 'viewer',    '只读成员', '可以读取和导出环境内的配置', TRUE, 0, 0),
  (2, 'editor',    '编辑成员', '可以编辑环境内的配置和导入快照', TRUE, 0, 0),
  (3, 'publisher', '发布成员', '包含编辑权限，并可以发布快照', TRUE, 0, 0),
  (4, 'admin',     '环境管理员', '包含全部环境级权限，并可以管理环境成员和项目密钥', TRUE, 0, 0);

-- viewer
INSERT INTO `role_permissions` (`role_id`, `permission_id`, `created_by`) VALUES
  (1, 1, 0), (1, 3, 0), (1, 5, 0), (1, 7, 0), (1, 9, 0), (1, 11, 0), (1, 14, 0);
-- editor = viewer + environment content mutation, import, and asset upload
INSERT INTO `role_permissions` (`role_id`, `permission_id`, `created_by`) VALUES
  (2, 1, 0), (2, 3, 0), (2, 5, 0), (2, 7, 0), (2, 9, 0), (2, 11, 0), (2, 14, 0),
  (2, 2, 0), (2, 6, 0), (2, 8, 0), (2, 10, 0), (2, 12, 0), (2, 15, 0), (2, 16, 0);
-- publisher = editor + publish
INSERT INTO `role_permissions` (`role_id`, `permission_id`, `created_by`) VALUES
  (3, 1, 0), (3, 3, 0), (3, 5, 0), (3, 7, 0), (3, 9, 0), (3, 11, 0), (3, 14, 0),
  (3, 2, 0), (3, 6, 0), (3, 8, 0), (3, 10, 0), (3, 12, 0), (3, 15, 0), (3, 16, 0),
  (3, 13, 0);
-- admin = publisher + project API Key management + environment membership
INSERT INTO `role_permissions` (`role_id`, `permission_id`, `created_by`) VALUES
  (4, 1, 0), (4, 3, 0), (4, 5, 0), (4, 7, 0), (4, 9, 0), (4, 11, 0), (4, 14, 0),
  (4, 2, 0), (4, 6, 0), (4, 8, 0), (4, 10, 0), (4, 12, 0), (4, 15, 0), (4, 16, 0),
  (4, 13, 0), (4, 4, 0), (4, 17, 0);

-- System permissions (18-20) are deliberately not assigned to an environment
-- role. They require users.is_system_admin and are checked separately.

COMMIT;
