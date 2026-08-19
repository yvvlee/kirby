import { afterEach, describe, expect, it } from 'vitest'

import {
  refreshAccessTokenSession,
  SESSION_CHANGED_DURING_REFRESH,
} from './refresh-coordinator'
import {
  clearAccessToken,
  getAccessTokenSnapshot,
  setAccessToken,
  startAccessTokenSession,
} from './token'

afterEach(() => {
  clearAccessToken()
})

describe('access token container', () => {
  it('separates access token and login session generations', () => {
    const initial = getAccessTokenSnapshot()

    startAccessTokenSession('first-token')
    const started = getAccessTokenSnapshot()
    setAccessToken('renewed-token')
    const renewed = getAccessTokenSnapshot()
    clearAccessToken()
    const cleared = getAccessTokenSnapshot()

    expect(started).toMatchObject({
      accessTokenGeneration: initial.accessTokenGeneration + 1,
      sessionGeneration: initial.sessionGeneration + 1,
    })
    expect(renewed).toMatchObject({
      token: 'renewed-token',
      accessTokenGeneration: started.accessTokenGeneration + 1,
      sessionGeneration: started.sessionGeneration,
    })
    expect(cleared).toMatchObject({
      token: null,
      sessionGeneration: renewed.sessionGeneration + 1,
      refreshAllowed: false,
      sessionActive: false,
    })
  })

  it('rejects a refresh result after the session changes', async () => {
    startAccessTokenSession('old-session-token')
    let resolveRefresh: ((reply: { access_token: string }) => void) | undefined
    const refreshReply = new Promise<{ access_token: string }>((resolve) => {
      resolveRefresh = resolve
    })
    const refreshing = refreshAccessTokenSession(() => refreshReply)

    clearAccessToken()
    resolveRefresh?.({ access_token: 'must-not-be-installed' })

    await expect(refreshing).rejects.toMatchObject({
      code: SESSION_CHANGED_DURING_REFRESH,
    })
    expect(getAccessTokenSnapshot()).toMatchObject({
      token: null,
      refreshAllowed: false,
      sessionActive: false,
    })
  })

  it('shares one refresh operation between concurrent callers', async () => {
    startAccessTokenSession('expired-token')
    let calls = 0
    let release: (() => void) | undefined
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const execute = async () => {
      calls += 1
      await gate
      return { access_token: 'renewed-token' }
    }

    const first = refreshAccessTokenSession(execute)
    const second = refreshAccessTokenSession(execute)
    release?.()

    await Promise.all([first, second])
    expect(calls).toBe(1)
    expect(getAccessTokenSnapshot().token).toBe('renewed-token')
  })
})
