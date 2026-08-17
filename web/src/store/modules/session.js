import {
  login as loginRequest,
  logout as logoutRequest,
  refreshSession,
} from '@/api/auth'
import {
  clearAccessToken,
  getAccessToken,
  getAccessTokenSnapshot,
  startAccessTokenSession,
} from '@/auth/token'
import { SESSION_CHANGED_DURING_REFRESH } from '@/store/refresh-coordinator'

const bootstrapOperations = new WeakMap()

function initialState() {
  return {
    user: null,
    initialized: false,
    busy: false,
  }
}

function requireLoginReply(reply) {
  if (typeof reply?.access_token !== 'string' || !reply.access_token) {
    throw new TypeError('authentication response does not contain access_token')
  }
  if (!reply.user || typeof reply.user !== 'object') {
    throw new TypeError('authentication response does not contain user')
  }
  return reply
}

function applyLoginReply(commit, reply, { tokenReady = false } = {}) {
  requireLoginReply(reply)
  if (tokenReady) {
    if (getAccessToken() !== reply.access_token) {
      throw new Error('refresh reply does not match the active access token')
    }
  } else {
    startAccessTokenSession(reply.access_token)
  }
  commit('SET_USER', reply.user)
}

function isAnonymousResponse(error) {
  return error?.response?.status === 401 || error?.response?.status === 403
}

export default {
  namespaced: true,

  state: initialState,

  getters: {
    authenticated: (state) => Boolean(state.user && getAccessToken()),
    systemAdmin: (state) => Boolean(state.user?.is_system_admin),
  },

  mutations: {
    SET_USER(state, user) {
      state.user = user
    },
    SET_INITIALIZED(state, initialized) {
      state.initialized = initialized
    },
    SET_BUSY(state, busy) {
      state.busy = busy
    },
  },

  actions: {
    async login({ commit, dispatch }, credentials) {
      commit('SET_BUSY', true)
      try {
        applyLoginReply(commit, await loginRequest(credentials))
        await dispatch('environment/loadAvailable', null, { root: true })
        commit('SET_INITIALIZED', true)
        return true
      } catch (error) {
        clearAccessToken()
        commit('SET_USER', null)
        await dispatch('environment/resetScope', null, { root: true })
        throw error
      } finally {
        commit('SET_BUSY', false)
      }
    },

    bootstrap(context) {
      const { state } = context
      if (state.initialized) {
        return Boolean(state.user && getAccessToken())
      }
      const pending = bootstrapOperations.get(state)
      if (pending) {
        return pending
      }

      const operation = (async () => {
        const { commit, dispatch } = context
        const startedSessionGeneration =
          getAccessTokenSnapshot().sessionGeneration
        let refreshedSessionGeneration = null
        try {
          const reply = await refreshSession()
          refreshedSessionGeneration =
            getAccessTokenSnapshot().sessionGeneration
          applyLoginReply(commit, reply, {
            tokenReady: true,
          })
          await dispatch('environment/loadAvailable', null, { root: true })
          commit('SET_INITIALIZED', true)
          return true
        } catch (error) {
          const current = getAccessTokenSnapshot()
          if (
            current.sessionGeneration !== startedSessionGeneration &&
            current.sessionGeneration !== refreshedSessionGeneration &&
            current.sessionActive
          ) {
            return Boolean(state.user && current.token)
          }
          if (
            (current.sessionGeneration === startedSessionGeneration ||
              current.sessionGeneration === refreshedSessionGeneration) &&
            (current.sessionActive || current.refreshAllowed)
          ) {
            clearAccessToken()
          }
          commit('SET_USER', null)
          await dispatch('environment/resetScope', null, { root: true })
          commit('SET_INITIALIZED', true)
          if (
            error?.code === SESSION_CHANGED_DURING_REFRESH ||
            isAnonymousResponse(error)
          ) {
            return false
          }
          throw error
        } finally {
          bootstrapOperations.delete(state)
        }
      })()
      bootstrapOperations.set(state, operation)
      return operation
    },

    async logout({ commit, dispatch }) {
      const logoutOperation = logoutRequest(getAccessToken())
      clearAccessToken()
      commit('SET_USER', null)
      commit('SET_INITIALIZED', true)
      const resetOperation = dispatch('environment/resetScope', null, {
        root: true,
      })
      try {
        await logoutOperation
      } finally {
        await resetOperation
      }
    },

    async expire({ commit, dispatch }) {
      const current = getAccessTokenSnapshot()
      if (current.sessionActive || current.refreshAllowed) {
        clearAccessToken()
      }
      commit('SET_USER', null)
      commit('SET_INITIALIZED', true)
      await dispatch('environment/resetScope', null, { root: true })
    },
  },
}
