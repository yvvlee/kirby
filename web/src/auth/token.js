let accessToken = null
const sessionExpiredListeners = new Set()

export function getAccessToken() {
  return accessToken
}

export function setAccessToken(token) {
  if (typeof token !== 'string' || token.length === 0) {
    throw new TypeError('access token must be a non-empty string')
  }
  accessToken = token
}

export function clearAccessToken() {
  accessToken = null
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
