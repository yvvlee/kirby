export type EnvironmentScopeChange = {
  fromEnvironmentId: number | string | null
  toEnvironmentId: number | string | null
}

export type EnvironmentCleanupHandler = (
  context: EnvironmentScopeChange,
) => void | Promise<void>

const cleanupHandlers = new Map<string, EnvironmentCleanupHandler>()

export function registerEnvironmentScopeCleanup(
  name: string,
  handler: EnvironmentCleanupHandler,
): () => void {
  if (name.length === 0) {
    throw new TypeError('environment cleanup name must be a non-empty string')
  }
  if (cleanupHandlers.has(name)) {
    throw new Error(`environment cleanup handler already registered: ${name}`)
  }
  cleanupHandlers.set(name, handler)
  return () => {
    cleanupHandlers.delete(name)
  }
}

export async function clearEnvironmentScope(
  context: EnvironmentScopeChange,
): Promise<void> {
  for (const [name, handler] of cleanupHandlers) {
    try {
      await handler(context)
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : String(error)
      throw new Error(`environment cleanup failed (${name}): ${message}`, {
        cause: error,
      })
    }
  }
}
