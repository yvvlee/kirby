import { postEnvironmentOperation } from './environment-resource'
import type { ApiEntity, ApiListReply, ApiObject, Identifier } from './types'

export const createModel = (environmentId: Identifier, model: ApiObject) =>
  postEnvironmentOperation<ApiEntity>(environmentId, 'structure', 'create', model)

export const updateModel = (environmentId: Identifier, model: ApiObject) =>
  postEnvironmentOperation<ApiEntity>(environmentId, 'structure', 'update', model)

export const listModels = (environmentId: Identifier, filter: ApiObject) =>
  postEnvironmentOperation<ApiListReply>(environmentId, 'structure', 'list', filter)

export const deleteModel = (environmentId: Identifier, modelId: Identifier) =>
  postEnvironmentOperation<ApiObject>(environmentId, 'structure', 'delete', {
    id: modelId,
  })
