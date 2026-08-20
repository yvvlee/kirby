import type { Identifier } from '@/api/types'

export const queryKeys = {
  environments: ['environments'] as const,
  users: ['users'] as const,
  roles: ['roles'] as const,
  permissions: ['permissions'] as const,
  globalProjects: (filter?: unknown) => filter === undefined
    ? ['projects'] as const
    : ['projects', filter] as const,
  environment: (environmentId: Identifier) =>
    ['environment', String(environmentId)] as const,
  environmentPermissions: (environmentId: Identifier) =>
    ['environment', String(environmentId), 'permissions'] as const,
  environmentMembers: (environmentId: Identifier) =>
    ['environment', String(environmentId), 'members'] as const,
  projects: (environmentId: Identifier, filter?: unknown) =>
    filter === undefined
      ? ['environment', String(environmentId), 'projects'] as const
      : ['environment', String(environmentId), 'projects', filter] as const,
  project: (environmentId: Identifier, projectId: Identifier) =>
    ['environment', String(environmentId), 'project', String(projectId)] as const,
  configs: (
    environmentId: Identifier,
    projectId: Identifier,
    filter?: unknown,
  ) => filter === undefined
    ? ['environment', String(environmentId), 'project', String(projectId), 'configs'] as const
    : ['environment', String(environmentId), 'project', String(projectId), 'configs', filter] as const,
  config: (
    environmentId: Identifier,
    projectId: Identifier,
    configId: Identifier,
  ) => ['environment', String(environmentId), 'project', String(projectId), 'config', String(configId)] as const,
  models: (
    environmentId: Identifier,
    projectId: Identifier,
    configId: Identifier,
    filter?: unknown,
  ) => filter === undefined
    ? ['environment', String(environmentId), 'project', String(projectId), 'config', String(configId), 'models'] as const
    : ['environment', String(environmentId), 'project', String(projectId), 'config', String(configId), 'models', filter] as const,
  enums: (
    environmentId: Identifier,
    projectId: Identifier,
    configId: Identifier,
    filter?: unknown,
  ) => filter === undefined
    ? ['environment', String(environmentId), 'project', String(projectId), 'config', String(configId), 'enums'] as const
    : ['environment', String(environmentId), 'project', String(projectId), 'config', String(configId), 'enums', filter] as const,
  snapshots: (
    environmentId: Identifier,
    projectId: Identifier,
    configId: Identifier,
    filter?: unknown,
  ) => filter === undefined
    ? ['environment', String(environmentId), 'project', String(projectId), 'config', String(configId), 'snapshots'] as const
    : ['environment', String(environmentId), 'project', String(projectId), 'config', String(configId), 'snapshots', filter] as const,
  apiKeys: (
    environmentId: Identifier,
    projectId: Identifier,
  ) =>
    [
      'environment',
      String(environmentId),
      'project',
      String(projectId),
      'api-keys',
    ] as const,
}

export function isEnvironmentQuery(queryKey: readonly unknown[]): boolean {
  return queryKey[0] === 'environment'
}
