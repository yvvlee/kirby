import {
  clearAccessToken,
  getAccessTokenSnapshot,
  notifySessionExpired,
  setAccessToken,
  startAccessTokenSession,
} from '@/auth/token'

let refreshOperation = null
let expirationOperation = null
let expiredSessionGeneration = null

export const SESSION_CHANGED_DURING_REFRESH =
  'KIRBY_SESSION_CHANGED_DURING_REFRESH'

function sessionChangedError() {
  const error = new Error('session changed while refresh was in flight')
  error.code = SESSION_CHANGED_DURING_REFRESH
  return error
}

function requireRefreshReply(reply) {
  const token = reply?.access_token
  if (typeof token !== 'string' || token.length === 0) {
    throw new TypeError('refresh response does not contain access_token')
  }
  return token
}

function unchangedSince(snapshot) {
  const current = getAccessTokenSnapshot()
  return (
    current.accessTokenGeneration === snapshot.accessTokenGeneration &&
    current.sessionGeneration === snapshot.sessionGeneration
  )
}

export async function expireAccessTokenSession(
  error,
  expectedSessionGeneration,
) {
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
    return expirationOperation?.promise
  }
  if (
    expirationOperation &&
    (expirationOperation.sourceSessionGeneration ===
      current.sessionGeneration ||
      expirationOperation.expiredSessionGeneration ===
        current.sessionGeneration)
  ) {
    return expirationOperation.promise
  }

  const sourceSessionGeneration = current.sessionGeneration
  clearAccessToken()
  const expiredGeneration = getAccessTokenSnapshot().sessionGeneration
  expiredSessionGeneration = expiredGeneration

  const operation = {
    sourceSessionGeneration,
    expiredSessionGeneration: expiredGeneration,
    promise: null,
  }
  operation.promise = Promise.resolve()
    .then(() => notifySessionExpired(error))
    .finally(() => {
      if (expirationOperation === operation) {
        expirationOperation = null
      }
    })
  expirationOperation = operation
  return operation.promise
}

export function refreshAccessTokenSession(execute) {
  if (typeof execute !== 'function') {
    throw new TypeError('refresh executor must be a function')
  }
  const started = getAccessTokenSnapshot()
  if (
    refreshOperation &&
    (unchangedSince(refreshOperation.started) ||
      (refreshOperation.completed &&
        unchangedSince(refreshOperation.completed)))
  ) {
    return refreshOperation.promise
  }
  if (!started.refreshAllowed) {
    return Promise.reject(new Error('access token refresh is not allowed'))
  }
  const operation = {
    started,
    completed: null,
    promise: null,
  }
  operation.promise = Promise.resolve()
    .then(execute)
    .then((reply) => {
      const token = requireRefreshReply(reply)
      if (!unchangedSince(started)) {
        throw sessionChangedError()
      }
      if (started.token === null) {
        startAccessTokenSession(token)
      } else {
        setAccessToken(token)
      }
      const result = {
        reply,
        started,
        completed: getAccessTokenSnapshot(),
      }
      operation.completed = result.completed
      return result
    })
    .catch(async (error) => {
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
