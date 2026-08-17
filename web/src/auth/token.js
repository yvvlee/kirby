let accessToken = null
let accessTokenGeneration = 0
let sessionGeneration = 0
let refreshAllowed = true
let sessionActive = false
const sessionExpiredListeners = new Set()

function requireAccessToken(token) {
  if (typeof token !== 'string' || token.length === 0) {
    throw new TypeError('access token must be a non-empty string')
  }
  return token
}

export function getAccessToken() {
  return accessToken
}

export function getAccessTokenSnapshot() {
  return Object.freeze({
    token: accessToken,
    accessTokenGeneration,
    sessionGeneration,
    refreshAllowed,
    sessionActive,
  })
}

export function setAccessToken(token) {
  if (!sessionActive) {
    throw new Error('cannot rotate an access token without an active session')
  }
  accessToken = requireAccessToken(token)
  accessTokenGeneration += 1
  refreshAllowed = true
}

export function startAccessTokenSession(token) {
  accessToken = requireAccessToken(token)
  accessTokenGeneration += 1
  sessionGeneration += 1
  refreshAllowed = true
  sessionActive = true
}

export function clearAccessToken() {
  accessToken = null
  accessTokenGeneration += 1
  sessionGeneration += 1
  refreshAllowed = false
  sessionActive = false
}

export function onSessionExpired(listener) {
  if (typeof listener !== 'function') {
    throw new TypeError('session expired listener must be a function')
  }
  sessionExpiredListeners.add(listener)
  return () => sessionExpiredListeners.delete(listener)
}

export async function notifySessionExpired(error) {
  for (const listener of sessionExpiredListeners) {
    await listener(error)
  }
}
