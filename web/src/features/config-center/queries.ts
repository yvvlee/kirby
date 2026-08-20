import { useQuery } from '@tanstack/react-query'

import { getConfig, listConfigs } from '@/api/configs'
import { listEnums } from '@/api/enums'
import { listModels } from '@/api/models'
import { getProject, listProjects } from '@/api/projects'
import { isIdentifier, type ApiEntity, type Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import type { EnumResource, ModelField } from '@/domain/schema'
import { normalizeModel } from '@/domain/type-codec'
import { normalizeTree } from '@/domain/type-codec'
import { requireList } from '@/features/system/errors'

export type Project = ApiEntity & {
  key: string
  name: string
  description?: string
  version: number
  updatedAt?: string
}

export type ConfigSummary = ApiEntity & {
  key: string
  description?: string
  version: number
  updatedAt?: string
  isReleased: boolean
}

export type ConfigDetail = ApiEntity & {
  key: string
  description?: string
  version: number
  updatedAt?: string
  type: Record<string, unknown>
  isArray: boolean
  value: string
}

export type ConfigDetailReply = {
  config: ConfigDetail
  tree: ReturnType<typeof normalizeTree> | null
}

export type ConfigModel = ApiEntity & {
  key: string
  name: string
  description?: string
  version: number
  updatedAt?: string
  fields: ModelField[]
}

export type ConfigEnumValue = { label: string; value: string; description?: string }
export type ConfigEnum = ApiEntity & EnumResource & {
  name: string
  description?: string
  version: number
  updatedAt?: string
  values: ConfigEnumValue[]
}

function requireObject(value: unknown, message: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new TypeError(message)
  return value as Record<string, unknown>
}

export function normalizeProjectDetail(reply: unknown): Project {
  const source = requireObject(reply, '项目详情响应缺少 project')
  const project = requireObject(source.project, '项目详情响应缺少 project')
  if (!isIdentifier(project.id) || typeof project.key !== 'string' || typeof project.name !== 'string') {
    throw new TypeError('项目详情响应中的 project 不完整')
  }
  return project as Project
}

export function normalizeConfigList(reply: unknown): ConfigSummary[] {
  return requireList<Record<string, unknown>>(reply, 'config list').map((item) => {
    const config = requireObject(item.config, '配置列表项缺少 config')
    if (!isIdentifier(config.id) || typeof config.key !== 'string') throw new TypeError('配置列表项中的 config 不完整')
    return { ...config, isReleased: Boolean(item.isReleased) } as ConfigSummary
  })
}

export function normalizeConfigDetail(reply: unknown): ConfigDetailReply {
  const source = requireObject(reply, '配置详情响应缺少 config')
  const rawConfig = requireObject(source.config, '配置详情响应缺少 config')
  if (!isIdentifier(rawConfig.id) || typeof rawConfig.key !== 'string') throw new TypeError('配置详情响应中的 config 不完整')
  const type = requireObject(rawConfig.type, '配置详情响应缺少 type')
  const config = {
    ...rawConfig,
    type,
    isArray: Boolean(rawConfig.isArray ?? rawConfig.is_array),
    value: typeof rawConfig.value === 'string' ? rawConfig.value : '',
  } as ConfigDetail
  return { config, tree: source.tree ? normalizeTree(source.tree) : null }
}

export function normalizeModels(reply: unknown): ConfigModel[] {
  return requireList<unknown>(reply, 'model list').map((item) => normalizeModel(item) as ConfigModel)
}

export function normalizeEnums(reply: unknown): ConfigEnum[] {
  return requireList<Record<string, unknown>>(reply, 'enum list').map((item) => {
    if (!Array.isArray(item.values)) throw new TypeError('枚举响应缺少 values')
    return item as ConfigEnum
  })
}

export function useProjectsQuery(environmentId: Identifier | null, keyword: string) {
  const filter = { keyword }
  return useQuery({
    queryKey: environmentId === null ? queryKeys.globalProjects(filter) : queryKeys.projects(environmentId, filter),
    queryFn: async () => {
      return requireList<Project>(await listProjects(environmentId, filter), 'project list')
    },
    enabled: true,
  })
}

export function useProjectQuery(environmentId: Identifier | null, projectId: Identifier) {
  return useQuery({
    queryKey: environmentId === null ? ['environment', 'none', 'project', String(projectId)] : queryKeys.project(environmentId, projectId),
    queryFn: async () => {
      if (environmentId === null) throw new Error('当前没有可用环境')
      return normalizeProjectDetail(await getProject(environmentId, projectId))
    },
    enabled: environmentId !== null,
  })
}

export function useConfigsQuery(environmentId: Identifier | null, projectId: Identifier, key: string) {
  const filter = key ? { key } : {}
  return useQuery({
    queryKey: environmentId === null ? ['environment', 'none', 'project', String(projectId), 'configs', filter] : queryKeys.configs(environmentId, projectId, filter),
    queryFn: async () => {
      if (environmentId === null) throw new Error('当前没有可用环境')
      return normalizeConfigList(await listConfigs(environmentId, { ...filter, project_id: projectId }))
    },
    enabled: environmentId !== null,
  })
}

export function useModelsQuery(environmentId: Identifier | null, projectId: Identifier, configId: Identifier) {
  return useQuery({
    queryKey: environmentId === null ? ['environment', 'none', 'models'] : queryKeys.models(environmentId, projectId, configId),
    queryFn: async () => {
      if (environmentId === null) throw new Error('当前没有可用环境')
      return normalizeModels(await listModels(environmentId, { project_id: projectId, config_id: configId }))
    },
    enabled: environmentId !== null,
  })
}

export function useEnumsQuery(environmentId: Identifier | null, projectId: Identifier, configId: Identifier) {
  return useQuery({
    queryKey: environmentId === null ? ['environment', 'none', 'enums'] : queryKeys.enums(environmentId, projectId, configId),
    queryFn: async () => {
      if (environmentId === null) throw new Error('当前没有可用环境')
      return normalizeEnums(await listEnums(environmentId, { project_id: projectId, config_id: configId }))
    },
    enabled: environmentId !== null,
  })
}

export function useConfigDetailQuery(environmentId: Identifier | null, projectId: Identifier, configId: Identifier) {
  return useQuery({
    queryKey: environmentId === null ? ['environment', 'none', 'config', String(configId)] : queryKeys.config(environmentId, projectId, configId),
    queryFn: async () => {
      if (environmentId === null) throw new Error('当前没有可用环境')
      return normalizeConfigDetail(await getConfig(environmentId, configId))
    },
    enabled: environmentId !== null,
  })
}
