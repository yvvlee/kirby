export type AccessTokenSnapshot = Readonly<{
  token: string | null
  accessTokenGeneration: number
  sessionGeneration: number
  refreshAllowed: boolean
  sessionActive: boolean
}>

export type SessionExpiredListener = (
  error: unknown,
) => void | Promise<void>

let accessToken: string | null = null
let accessTokenGeneration = 0
let sessionGeneration = 0
let refreshAllowed = true
let sessionActive = false
const sessionExpiredListeners = new Set<SessionExpiredListener>()

function requireAccessToken(token: string): string {
  if (token.length === 0) {
    throw new TypeError('access token must be a non-empty string')
  }
  return token
}

export function getAccessToken(): string | null {
  return accessToken
}

export function getAccessTokenSnapshot(): AccessTokenSnapshot {
  return Object.freeze({
    token: accessToken,
    accessTokenGeneration,
    sessionGeneration,
    refreshAllowed,
    sessionActive,
  })
}

export function setAccessToken(token: string): void {
  if (!sessionActive) {
    throw new Error('cannot rotate an access token without an active session')
  }
  accessToken = requireAccessToken(token)
  accessTokenGeneration += 1
  refreshAllowed = true
}

export function startAccessTokenSession(token: string): void {
  accessToken = requireAccessToken(token)
  accessTokenGeneration += 1
  sessionGeneration += 1
  refreshAllowed = true
  sessionActive = true
}

export function clearAccessToken(): void {
  accessToken = null
  accessTokenGeneration += 1
  sessionGeneration += 1
  refreshAllowed = false
  sessionActive = false
}

export function onSessionExpired(listener: SessionExpiredListener): () => void {
  sessionExpiredListeners.add(listener)
  return () => {
    sessionExpiredListeners.delete(listener)
  }
}

export async function notifySessionExpired(error: unknown): Promise<void> {
  for (const listener of sessionExpiredListeners) {
    await listener(error)
  }
}
