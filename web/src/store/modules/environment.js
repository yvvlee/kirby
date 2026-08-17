import {
  getMyPermissions,
  listEnvironments,
} from '@/api/environments'
import { clearEnvironmentScope } from '@/auth/environment-scope'
import { getAccessTokenSnapshot } from '@/auth/token'

function initialState() {
  return {
    available: [],
    currentId: null,
    permissions: [],
    switchSequence: 0,
    switching: false,
    scopeVersion: 0,
    lifecycleVersion: 0,
  }
}

function sameId(left, right) {
  return String(left) === String(right)
}

function requireEnvironmentList(reply) {
  if (!Array.isArray(reply?.list)) {
    throw new TypeError('environment list response does not contain list')
  }
  return reply.list
}

function requirePermissions(reply) {
  if (!Array.isArray(reply?.permissions)) {
    throw new TypeError('permission response does not contain permissions')
  }
  return reply.permissions
}

export default {
  namespaced: true,

  state: initialState,

  getters: {
    current(state) {
      return (
        state.available.find((item) => sameId(item.id, state.currentId)) ||
        null
      )
    },
    hasPermission: (state) => (permission) =>
      state.permissions.includes(permission),
  },

  mutations: {
    SET_AVAILABLE(state, environments) {
      state.available = environments
    },
    BEGIN_SWITCH(state) {
      state.switchSequence += 1
      state.switching = true
    },
    COMPLETE_SWITCH(state, { environmentId, permissions }) {
      state.currentId = environmentId
      state.permissions = permissions
      state.switching = false
      state.scopeVersion += 1
    },
    CANCEL_SWITCH(state) {
      state.switching = false
    },
    RESET(state) {
      const switchSequence = state.switchSequence + 1
      const scopeVersion = state.scopeVersion + 1
      const lifecycleVersion = state.lifecycleVersion + 1
      Object.assign(state, initialState(), {
        switchSequence,
        scopeVersion,
        lifecycleVersion,
      })
    },
  },

  actions: {
    async loadAvailable({ state, commit, dispatch }) {
      const lifecycleVersion = state.lifecycleVersion
      const sessionGeneration =
        getAccessTokenSnapshot().sessionGeneration
      const environments = requireEnvironmentList(await listEnvironments())
      if (
        lifecycleVersion !== state.lifecycleVersion ||
        sessionGeneration !== getAccessTokenSnapshot().sessionGeneration
      ) {
        return []
      }
      commit('SET_AVAILABLE', environments)

      const current = environments.find(
        (item) => item.enabled && sameId(item.id, state.currentId),
      )
      const next = current || environments.find((item) => item.enabled)
      if (!next) {
        await dispatch('resetScope')
        return environments
      }

      await dispatch('select', next.id)
      return environments
    },

    async select({ state, commit, getters }, environmentId) {
      const environment = state.available.find((item) =>
        sameId(item.id, environmentId),
      )
      if (!environment) {
        throw new Error(`environment is not available: ${environmentId}`)
      }
      if (!environment.enabled) {
        throw new Error(`environment is disabled: ${environmentId}`)
      }

      commit('BEGIN_SWITCH')
      const sequence = state.switchSequence
      const lifecycleVersion = state.lifecycleVersion
      const sessionGeneration =
        getAccessTokenSnapshot().sessionGeneration
      try {
        const permissions = requirePermissions(
          await getMyPermissions(environment.id),
        )
        if (
          sequence !== state.switchSequence ||
          lifecycleVersion !== state.lifecycleVersion ||
          sessionGeneration !== getAccessTokenSnapshot().sessionGeneration
        ) {
          return null
        }

        const previous = getters.current
        const previousId = previous?.id ?? state.currentId
        if (previousId !== null && !sameId(previousId, environment.id)) {
          await clearEnvironmentScope({
            fromEnvironmentId: previousId,
            toEnvironmentId: environment.id,
          })
        }
        if (
          sequence !== state.switchSequence ||
          lifecycleVersion !== state.lifecycleVersion ||
          sessionGeneration !== getAccessTokenSnapshot().sessionGeneration
        ) {
          return null
        }

        commit('COMPLETE_SWITCH', {
          environmentId: environment.id,
          permissions,
        })
        return environment
      } catch (error) {
        if (
          sequence === state.switchSequence &&
          lifecycleVersion === state.lifecycleVersion &&
          sessionGeneration === getAccessTokenSnapshot().sessionGeneration
        ) {
          commit('CANCEL_SWITCH')
        }
        throw error
      }
    },

    async resetScope({ state, commit, getters }) {
      const previous = getters.current
      await clearEnvironmentScope({
        fromEnvironmentId: previous?.id ?? state.currentId,
        toEnvironmentId: null,
      })
      commit('RESET')
    },
  },
}
