-- Kirby schema for MySQL 8.0.
--
-- The service runs this embedded schema for an empty database and generates
-- installation secrets after schema creation. This file is also a schema
-- reference. IF NOT EXISTS is omitted so incomplete initialization fails early.

SET NAMES utf8mb4 COLLATE utf8mb4_0900_ai_ci;
SET time_zone = '+00:00';

CREATE TABLE `application_secrets` (
  `id` BIGINT UNSIGNED NOT NULL,
  `jwt_key_id` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `jwt_signing_key` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `api_key_pepper` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `admin_initialized` BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (`id`),
  CONSTRAINT `ck_application_secrets_singleton` CHECK (`id` = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `application_settings` (
  `id` BIGINT UNSIGNED NOT NULL,
  `allow_self_registration` BOOLEAN NOT NULL DEFAULT FALSE,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  CONSTRAINT `ck_application_settings_singleton` CHECK (`id` = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `environments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `key` VARCHAR(64) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `enabled` BOOLEAN NOT NULL DEFAULT TRUE,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_environments_project_key` (`project_id`, `key`),
  KEY `ix_environments_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(128) NOT NULL,
  `email` VARCHAR(320) NOT NULL,
  `nickname` VARCHAR(128) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `enabled` BOOLEAN NOT NULL DEFAULT TRUE,
  `is_system_admin` BOOLEAN NOT NULL DEFAULT FALSE,
  `must_change_password` BOOLEAN NOT NULL DEFAULT FALSE,
  `last_login_at` DATETIME(6) NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_users_username` (`username`),
  UNIQUE KEY `ux_users_email` (`email`),
  KEY `ix_users_enabled` (`enabled`),
  KEY `ix_users_system_admin` (`is_system_admin`, `enabled`)
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
  KEY `ix_refresh_tokens_replaced_by` (`replaced_by_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `projects` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `key` VARCHAR(128) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `updated_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_projects_key` (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `project_api_keys` (
  `environment_id` BIGINT UNSIGNED NOT NULL,
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `secret_hash` BINARY(32) NOT NULL,
  `secret_suffix` CHAR(4) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL,
  `last_used_at` DATETIME(6) NULL,
  `revoked_at` DATETIME(6) NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_project_api_keys_secret_hash` (`secret_hash`),
  KEY `ix_project_api_keys_project_active` (`project_id`, `revoked_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `configs` (
  `environment_id` BIGINT UNSIGNED NOT NULL,
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_configs_environment_key` (`environment_id`, `key`),
  KEY `ix_configs_runtime_version` (`project_id`, `key`, `runtime_version`),
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_structures_config_key` (`config_id`, `key`)
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_config_enums_config_key` (`config_id`, `key`)
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
  PRIMARY KEY (`id`),
  KEY `ix_snapshots_config_created` (`config_id`, `created_at`),
  KEY `ix_snapshots_project_config_status` (`project_id`, `config_id`, `status`),
  KEY `ix_snapshots_runtime_lookup` (`config_id`, `status`, `is_using`),
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


CREATE TABLE `project_members` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `role` VARCHAR(16) NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_project_members_project_user` (`project_id`, `user_id`),
  KEY `ix_project_members_user` (`user_id`),
  CONSTRAINT `ck_project_members_role` CHECK (`role` IN ('OWNER', 'MAINTAINER', 'MEMBER', 'GUEST'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `environment_permission_grants` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `environment_id` BIGINT UNSIGNED NOT NULL,
  `role` VARCHAR(16) NOT NULL,
  `permission` VARCHAR(32) NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_environment_permission_grant` (`environment_id`, `role`, `permission`),
  KEY `ix_environment_permission_grants_environment` (`environment_id`),
  CONSTRAINT `ck_environment_permission_grants_role` CHECK (`role` IN ('OWNER', 'MAINTAINER', 'MEMBER', 'GUEST')),
  CONSTRAINT `ck_environment_permission_grants_permission` CHECK (`permission` IN ('READ', 'WRITE', 'PUBLISH', 'API_KEY_MANAGE'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `project_invitations` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` BIGINT UNSIGNED NOT NULL,
  `role` VARCHAR(16) NOT NULL,
  `token_hash` BINARY(32) NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL,
  `expires_at` DATETIME(6) NOT NULL,
  `accepted_by` BIGINT UNSIGNED NULL,
  `accepted_at` DATETIME(6) NULL,
  `revoked_at` DATETIME(6) NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_project_invitations_token_hash` (`token_hash`),
  KEY `ix_project_invitations_project` (`project_id`),
  CONSTRAINT `ck_project_invitations_role` CHECK (`role` IN ('OWNER', 'MAINTAINER', 'MEMBER', 'GUEST'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO application_settings (id, allow_self_registration) VALUES (1, FALSE);
