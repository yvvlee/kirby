import Vue from 'vue'
import Vuex from 'vuex'

import configCenter from './modules/config-center'
import environment from './modules/environment'
import session from './modules/session'

Vue.use(Vuex)

export function createStore() {
  return new Vuex.Store({
    strict: import.meta.env.DEV,
    modules: {
      configCenter,
      environment,
      session,
    },
  })
}

const store = createStore()

export default store
