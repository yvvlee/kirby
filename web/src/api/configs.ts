import { postEnvironmentOperation } from './environment-resource'
import type { ApiEntity, ApiListReply, ApiObject, Identifier } from './types'

export const createConfig = (environmentId: Identifier, config: ApiObject) =>
  postEnvironmentOperation<ApiEntity>(environmentId, 'config', 'create', config)

export const updateConfig = (environmentId: Identifier, config: ApiObject) =>
  postEnvironmentOperation<ApiEntity>(environmentId, 'config', 'update', config)

export const updateConfigValue = (
  environmentId: Identifier,
  config: ApiObject,
) =>
  postEnvironmentOperation<ApiEntity>(
    environmentId,
    'config',
    'updateValue',
    config,
  )

export const listConfigs = (
  environmentId: Identifier,
  filter: ApiObject = {},
) =>
  postEnvironmentOperation<ApiListReply>(environmentId, 'config', 'list', filter)

export const getConfig = (environmentId: Identifier, configId: Identifier) =>
  postEnvironmentOperation<ApiEntity>(environmentId, 'config', 'detail', {
    id: configId,
  })

export const deleteConfig = (
  environmentId: Identifier,
  configId: Identifier,
) =>
  postEnvironmentOperation<ApiObject>(environmentId, 'config', 'delete', {
    id: configId,
  })
