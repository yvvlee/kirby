import {
  login as loginRequest,
  logout as logoutRequest,
  refreshSession,
} from '@/api/auth'
import {
  clearAccessToken,
  getAccessToken,
  setAccessToken,
} from '@/auth/token'

function initialState() {
  return {
    user: null,
    initialized: false,
    busy: false,
  }
}

function applyLoginReply(commit, reply) {
  if (typeof reply?.access_token !== 'string' || !reply.access_token) {
    throw new TypeError('authentication response does not contain access_token')
  }
  if (!reply.user || typeof reply.user !== 'object') {
    throw new TypeError('authentication response does not contain user')
  }
  setAccessToken(reply.access_token)
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

    async bootstrap({ state, commit, dispatch }) {
      if (state.initialized) {
        return Boolean(state.user && getAccessToken())
      }

      try {
        applyLoginReply(commit, await refreshSession())
        await dispatch('environment/loadAvailable', null, { root: true })
        commit('SET_INITIALIZED', true)
        return true
      } catch (error) {
        clearAccessToken()
        commit('SET_USER', null)
        await dispatch('environment/resetScope', null, { root: true })
        commit('SET_INITIALIZED', true)
        if (isAnonymousResponse(error)) {
          return false
        }
        throw error
      }
    },

    async logout({ commit, dispatch }) {
      try {
        await logoutRequest()
      } finally {
        clearAccessToken()
        commit('SET_USER', null)
        commit('SET_INITIALIZED', true)
        await dispatch('environment/resetScope', null, { root: true })
      }
    },

    async expire({ commit, dispatch }) {
      clearAccessToken()
      commit('SET_USER', null)
      commit('SET_INITIALIZED', true)
      await dispatch('environment/resetScope', null, { root: true })
    },
  },
}
