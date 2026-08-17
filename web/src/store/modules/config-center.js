import { listConfigs } from '@/api/configs'
import { listEnums } from '@/api/enums'
import { listModels } from '@/api/models'
import { listProjects } from '@/api/projects'
import { listSnapshots } from '@/api/snapshots'
import { registerEnvironmentScopeCleanup } from '@/auth/environment-scope'

const trackedCommits = new Set()

registerEnvironmentScopeCleanup(
  'config-center-cache',
  async ({ fromEnvironmentId }) => {
    for (const commit of trackedCommits) {
      commit('CLEAR_ENVIRONMENT', fromEnvironmentId)
    }
  },
)

function emptyBuckets() {
  return {
    configs: {},
    enums: {},
    models: {},
    projects: {},
    snapshots: {},
  }
}

function initialState() {
  return {
    ...emptyBuckets(),
    cleanupTracked: false,
    epoch: 0,
    generations: {},
  }
}

function positiveId(name, value) {
  const id = String(value)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return id
}

function environmentPrefix(environmentId) {
  return `environment:${positiveId('environmentId', environmentId)}|`
}

function scopeKey(environmentId, projectId = '*', configId = '*') {
  const environment = positiveId('environmentId', environmentId)
  const project =
    projectId === '*'
      ? '*'
      : positiveId('projectId', projectId)
  const config =
    configId === '*'
      ? '*'
      : positiveId('configId', configId)
  return `environment:${environment}|project:${project}|config:${config}`
}

function projectPrefix(environmentId, projectId) {
  const environment = positiveId('environmentId', environmentId)
  const project = positiveId('projectId', projectId)
  return `environment:${environment}|project:${project}|`
}

function canonical(value) {
  if (Array.isArray(value)) {
    return value.map(canonical)
  }
  if (value && typeof value === 'object') {
    return Object.keys(value)
      .sort()
      .reduce((result, key) => {
        if (value[key] !== undefined) {
          result[key] = canonical(value[key])
        }
        return result
      }, {})
  }
  return value
}

function signature(value) {
  return JSON.stringify(canonical(value))
}

function requireFilter(filter) {
  if (!filter || typeof filter !== 'object' || Array.isArray(filter)) {
    throw new TypeError('filter must be an object')
  }
  return filter
}

function requireListReply(resource, reply) {
  if (!reply || typeof reply !== 'object' || !Array.isArray(reply.list)) {
    throw new TypeError(`${resource} response does not contain list`)
  }
  return reply
}

function cacheHit(state, bucket, key, requestSignature, force) {
  const entry = state[bucket][key]
  if (!force && entry?.signature === requestSignature) {
    return entry.reply
  }
  return null
}

function trackCleanup(state, commit) {
  if (!state.cleanupTracked) {
    trackedCommits.add(commit)
    commit('MARK_CLEANUP_TRACKED')
  }
}

async function loadScopedList({
  state,
  commit,
  bucket,
  key,
  environmentId,
  filter,
  force,
  resource,
  request,
}) {
  trackCleanup(state, commit)
  const requestSignature = signature(filter)
  const cached = cacheHit(state, bucket, key, requestSignature, force)
  if (cached) {
    return cached
  }

  const environment = positiveId('environmentId', environmentId)
  const generation = `${state.epoch}:${state.generations[environment] || 0}`
  const reply = requireListReply(resource, await request())
  if (
    `${state.epoch}:${state.generations[environment] || 0}` !== generation
  ) {
    return reply
  }
  commit('CACHE_REPLY', {
    bucket,
    key,
    signature: requestSignature,
    reply,
  })
  return reply
}

function withoutEnvironment(entries, environmentId) {
  const prefix = environmentPrefix(environmentId)
  return Object.fromEntries(
    Object.entries(entries).filter(([key]) => !key.startsWith(prefix)),
  )
}

export default {
  namespaced: true,

  state: initialState,

  getters: {
    projects: (state) => (environmentId) =>
      state.projects[scopeKey(environmentId)]?.reply.list || [],
    configs: (state) => (environmentId, projectId) =>
      state.configs[scopeKey(environmentId, projectId)]?.reply.list || [],
    models: (state) => (environmentId, projectId, configId) =>
      state.models[scopeKey(environmentId, projectId, configId)]?.reply.list ||
      [],
    enums: (state) => (environmentId, projectId, configId) =>
      state.enums[scopeKey(environmentId, projectId, configId)]?.reply.list ||
      [],
    snapshots: (state) => (environmentId, projectId, configId) =>
      state.snapshots[scopeKey(environmentId, projectId, configId)]?.reply
        .list || [],
    snapshotPage: (state) => (environmentId, projectId, configId) =>
      state.snapshots[scopeKey(environmentId, projectId, configId)]?.reply
        .page || null,
  },

  mutations: {
    MARK_CLEANUP_TRACKED(state) {
      state.cleanupTracked = true
    },
    CACHE_REPLY(state, { bucket, key, signature: requestSignature, reply }) {
      if (!Object.hasOwn(state, bucket) || bucket === 'generations') {
        throw new TypeError(`unknown config center cache bucket: ${bucket}`)
      }
      state[bucket] = {
        ...state[bucket],
        [key]: { signature: requestSignature, reply },
      }
    },
    CLEAR_PROJECT(state, { environmentId, projectId }) {
      const prefix = projectPrefix(environmentId, projectId)
      for (const bucket of ['configs', 'enums', 'models', 'snapshots']) {
        state[bucket] = Object.fromEntries(
          Object.entries(state[bucket]).filter(
            ([key]) => !key.startsWith(prefix),
          ),
        )
      }
    },
    CLEAR_ENVIRONMENT(state, environmentId) {
      const environment = positiveId('environmentId', environmentId)
      for (const bucket of [
        'configs',
        'enums',
        'models',
        'projects',
        'snapshots',
      ]) {
        state[bucket] = withoutEnvironment(state[bucket], environment)
      }
      state.generations = {
        ...state.generations,
        [environment]: (state.generations[environment] || 0) + 1,
      }
    },
    RESET(state) {
      const generations = Object.fromEntries(
        Object.entries(state.generations).map(([key, value]) => [
          key,
          value + 1,
        ]),
      )
      Object.assign(state, {
        ...emptyBuckets(),
        cleanupTracked: state.cleanupTracked,
        epoch: state.epoch + 1,
        generations,
      })
    },
  },

  actions: {
    loadProjects(
      { state, commit },
      { environmentId, filter = {}, force = false },
    ) {
      filter = requireFilter(filter)
      return loadScopedList({
        state,
        commit,
        bucket: 'projects',
        key: scopeKey(environmentId),
        environmentId,
        filter,
        force,
        resource: 'project list',
        request: () => listProjects(environmentId, filter),
      })
    },

    loadConfigs(
      { state, commit },
      { environmentId, projectId, filter = {}, force = false },
    ) {
      filter = requireFilter(filter)
      positiveId('projectId', projectId)
      const requestFilter = { ...filter, project_id: projectId }
      return loadScopedList({
        state,
        commit,
        bucket: 'configs',
        key: scopeKey(environmentId, projectId),
        environmentId,
        filter: requestFilter,
        force,
        resource: 'config list',
        request: () => listConfigs(environmentId, requestFilter),
      })
    },

    loadModels(
      { state, commit },
      { environmentId, projectId, configId, filter = {}, force = false },
    ) {
      filter = requireFilter(filter)
      positiveId('projectId', projectId)
      positiveId('configId', configId)
      const requestFilter = {
        ...filter,
        project_id: projectId,
        config_id: configId,
      }
      return loadScopedList({
        state,
        commit,
        bucket: 'models',
        key: scopeKey(environmentId, projectId, configId),
        environmentId,
        filter: requestFilter,
        force,
        resource: 'model list',
        request: () => listModels(environmentId, requestFilter),
      })
    },

    loadEnums(
      { state, commit },
      { environmentId, projectId, configId, filter = {}, force = false },
    ) {
      filter = requireFilter(filter)
      positiveId('projectId', projectId)
      positiveId('configId', configId)
      const requestFilter = {
        ...filter,
        project_id: projectId,
        config_id: configId,
      }
      return loadScopedList({
        state,
        commit,
        bucket: 'enums',
        key: scopeKey(environmentId, projectId, configId),
        environmentId,
        filter: requestFilter,
        force,
        resource: 'enum list',
        request: () => listEnums(environmentId, requestFilter),
      })
    },

    loadSnapshots(
      { state, commit },
      { environmentId, projectId, configId, filter = {}, force = false },
    ) {
      filter = requireFilter(filter)
      positiveId('projectId', projectId)
      positiveId('configId', configId)
      const requestFilter = {
        ...filter,
        project_id: projectId,
        config_id: configId,
      }
      return loadScopedList({
        state,
        commit,
        bucket: 'snapshots',
        key: scopeKey(environmentId, projectId, configId),
        environmentId,
        filter: requestFilter,
        force,
        resource: 'snapshot list',
        request: () => listSnapshots(environmentId, requestFilter),
      })
    },

    invalidateProject({ commit }, { environmentId, projectId }) {
      commit('CLEAR_PROJECT', { environmentId, projectId })
    },
    invalidateEnvironment({ commit }, environmentId) {
      commit('CLEAR_ENVIRONMENT', environmentId)
    },
    reset({ commit }) {
      commit('RESET')
    },
  },
}
