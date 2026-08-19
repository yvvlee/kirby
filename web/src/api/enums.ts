import { postEnvironmentOperation } from './environment-resource'
import type { ApiEntity, ApiListReply, ApiObject, Identifier } from './types'

export const createEnum = (
  environmentId: Identifier,
  configEnum: ApiObject,
) => postEnvironmentOperation<ApiEntity>(environmentId, 'enum', 'create', configEnum)

export const updateEnum = (
  environmentId: Identifier,
  configEnum: ApiObject,
) => postEnvironmentOperation<ApiEntity>(environmentId, 'enum', 'update', configEnum)

export const listEnums = (environmentId: Identifier, filter: ApiObject) =>
  postEnvironmentOperation<ApiListReply>(environmentId, 'enum', 'list', filter)

export const deleteEnum = (environmentId: Identifier, enumId: Identifier) =>
  postEnvironmentOperation<ApiObject>(environmentId, 'enum', 'delete', {
    id: enumId,
  })
