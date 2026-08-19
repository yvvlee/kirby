import {
  clearAccessToken,
  getAccessTokenSnapshot,
  notifySessionExpired,
  setAccessToken,
  startAccessTokenSession,
  type AccessTokenSnapshot,
} from './token'

export const SESSION_CHANGED_DURING_REFRESH =
  'KIRBY_SESSION_CHANGED_DURING_REFRESH'

export type RefreshResult = Readonly<{
  reply: unknown
  started: AccessTokenSnapshot
  completed: AccessTokenSnapshot
}>

type RefreshOperation = {
  started: AccessTokenSnapshot
  completed: AccessTokenSnapshot | null
  promise: Promise<RefreshResult>
}

type ExpirationOperation = {
  sourceSessionGeneration: number
  expiredSessionGeneration: number
  promise: Promise<void>
}

let refreshOperation: RefreshOperation | null = null
let expirationOperation: ExpirationOperation | null = null
let expiredSessionGeneration: number | null = null

function sessionChangedError(): Error & { code: string } {
  return Object.assign(
    new Error('session changed while refresh was in flight'),
    { code: SESSION_CHANGED_DURING_REFRESH },
  )
}

function requireRefreshToken(reply: unknown): string {
  if (
    typeof reply !== 'object' ||
    reply === null ||
    !('access_token' in reply) ||
    typeof reply.access_token !== 'string' ||
    reply.access_token.length === 0
  ) {
    throw new TypeError('refresh response does not contain access_token')
  }
  return reply.access_token
}

function unchangedSince(snapshot: AccessTokenSnapshot): boolean {
  const current = getAccessTokenSnapshot()
  return (
    current.accessTokenGeneration === snapshot.accessTokenGeneration &&
    current.sessionGeneration === snapshot.sessionGeneration
  )
}

export async function expireAccessTokenSession(
  error: unknown,
  expectedSessionGeneration?: number,
): Promise<void> {
  const current = getAccessTokenSnapshot()
  if (
    expectedSessionGeneration !== undefined &&
    current.sessionGeneration !== expectedSessionGeneration
  ) {
    return
  }
  if (
    !current.refreshAllowed &&
    !current.sessionActive &&
    expiredSessionGeneration === current.sessionGeneration
  ) {
    await expirationOperation?.promise
    return
  }
  if (
    expirationOperation &&
    (expirationOperation.sourceSessionGeneration === current.sessionGeneration ||
      expirationOperation.expiredSessionGeneration === current.sessionGeneration)
  ) {
    await expirationOperation.promise
    return
  }

  const sourceSessionGeneration = current.sessionGeneration
  clearAccessToken()
  const expiredGeneration = getAccessTokenSnapshot().sessionGeneration
  expiredSessionGeneration = expiredGeneration

  const operation: ExpirationOperation = {
    sourceSessionGeneration,
    expiredSessionGeneration: expiredGeneration,
    promise: Promise.resolve(),
  }
  operation.promise = Promise.resolve()
    .then(() => notifySessionExpired(error))
    .finally(() => {
      if (expirationOperation === operation) {
        expirationOperation = null
      }
    })
  expirationOperation = operation
  await operation.promise
}

export function refreshAccessTokenSession(
  execute: () => unknown | Promise<unknown>,
): Promise<RefreshResult> {
  const started = getAccessTokenSnapshot()
  if (
    refreshOperation &&
    (unchangedSince(refreshOperation.started) ||
      (refreshOperation.completed !== null &&
        unchangedSince(refreshOperation.completed)))
  ) {
    return refreshOperation.promise
  }
  if (!started.refreshAllowed) {
    return Promise.reject(new Error('access token refresh is not allowed'))
  }

  const operation = {} as RefreshOperation
  operation.started = started
  operation.completed = null
  operation.promise = Promise.resolve()
    .then(execute)
    .then((reply) => {
      const token = requireRefreshToken(reply)
      if (!unchangedSince(started)) {
        throw sessionChangedError()
      }
      if (started.token === null) {
        startAccessTokenSession(token)
      } else {
        setAccessToken(token)
      }
      const result = Object.freeze({
        reply,
        started,
        completed: getAccessTokenSnapshot(),
      })
      operation.completed = result.completed
      return result
    })
    .catch(async (error: unknown) => {
      if (unchangedSince(started)) {
        await expireAccessTokenSession(error, started.sessionGeneration)
      }
      throw error
    })
    .finally(() => {
      if (refreshOperation === operation) {
        refreshOperation = null
      }
    })
  refreshOperation = operation
  return operation.promise
}
