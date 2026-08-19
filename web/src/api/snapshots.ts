import { postEnvironmentOperation } from './environment-resource'
import type { ApiEntity, ApiListReply, ApiObject, Identifier } from './types'

export const createSnapshot = (
  environmentId: Identifier,
  snapshot: ApiObject,
) => postEnvironmentOperation<ApiEntity>(environmentId, 'snapshot', 'create', snapshot)

export const previewCreatingSnapshot = (
  environmentId: Identifier,
  configId: Identifier,
) =>
  postEnvironmentOperation<ApiObject>(environmentId, 'snapshot', 'previewCreating', {
    config_id: configId,
  })

export const deleteSnapshot = (
  environmentId: Identifier,
  snapshotId: Identifier,
) => postEnvironmentOperation<ApiObject>(environmentId, 'snapshot', 'delete', { id: snapshotId })

export const getSnapshot = (
  environmentId: Identifier,
  snapshotId: Identifier,
) => postEnvironmentOperation<ApiEntity>(environmentId, 'snapshot', 'detail', { id: snapshotId })

export const loadSnapshot = (
  environmentId: Identifier,
  snapshotId: Identifier,
  configId: Identifier,
) =>
  postEnvironmentOperation<ApiObject>(environmentId, 'snapshot', 'load', {
    id: snapshotId,
    config_id: configId,
  })

export const getCurrentSnapshot = (
  environmentId: Identifier,
  configId: Identifier,
) => postEnvironmentOperation<ApiEntity>(environmentId, 'snapshot', 'current', { config_id: configId })

export const getReleasedSnapshot = (
  environmentId: Identifier,
  configId: Identifier,
) => postEnvironmentOperation<ApiEntity>(environmentId, 'snapshot', 'released', { config_id: configId })

export const listSnapshots = (
  environmentId: Identifier,
  filter: ApiObject,
) => postEnvironmentOperation<ApiListReply>(environmentId, 'snapshot', 'list', filter)
