const cleanupHandlers = new Map()

export function registerEnvironmentScopeCleanup(name, handler) {
  if (typeof name !== 'string' || name.length === 0) {
    throw new TypeError('environment cleanup name must be a non-empty string')
  }
  if (typeof handler !== 'function') {
    throw new TypeError('environment cleanup handler must be a function')
  }
  if (cleanupHandlers.has(name)) {
    throw new Error(`environment cleanup handler already registered: ${name}`)
  }

  cleanupHandlers.set(name, handler)
  return () => cleanupHandlers.delete(name)
}

export async function clearEnvironmentScope(context) {
  for (const [name, handler] of cleanupHandlers) {
    try {
      await handler(context)
    } catch (error) {
      error.message = `environment cleanup failed (${name}): ${error.message}`
      throw error
    }
  }
}
