import { onSessionExpired } from '@/auth/token'

function requiresAuthentication(route) {
  return route.matched.some((record) => record.meta.requiresAuth)
}

function requiredPermissions(route) {
  return route.matched.flatMap((record) => {
    if (record.meta.permission) {
      return [record.meta.permission]
    }
    return record.meta.permissions || []
  })
}

function safeRedirect(value) {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//')
    ? value
    : '/'
}

export function createNavigationGuard(store) {
  return async (to, from, next) => {
    await store.dispatch('session/bootstrap')

    const authenticated = store.getters['session/authenticated']
    if (to.name === 'login') {
      next(authenticated ? safeRedirect(to.query.redirect) : undefined)
      return
    }

    if (requiresAuthentication(to) && !authenticated) {
      next({
        name: 'login',
        query: { redirect: to.fullPath },
      })
      return
    }

    const permissions = requiredPermissions(to)
    const systemAdmin = store.getters['session/systemAdmin']
    const hasPermission = store.getters['environment/hasPermission']
    if (
      authenticated &&
      !systemAdmin &&
      permissions.some((permission) => !hasPermission(permission))
    ) {
      next({ name: 'forbidden' })
      return
    }

    next()
  }
}

export function installRouterGuards(router, store) {
  router.beforeEach(createNavigationGuard(store))

  return onSessionExpired(async () => {
    await store.dispatch('session/expire')
    if (router.currentRoute.name !== 'login') {
      await router.replace({
        name: 'login',
        query: { redirect: router.currentRoute.fullPath },
      })
    }
  })
}
