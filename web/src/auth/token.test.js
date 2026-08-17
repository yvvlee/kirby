import { afterEach, describe, expect, it } from 'vitest'

import {
  clearAccessToken,
  getAccessTokenSnapshot,
  setAccessToken,
  startAccessTokenSession,
} from './token'
import {
  refreshAccessTokenSession,
  SESSION_CHANGED_DURING_REFRESH,
} from '@/store/refresh-coordinator'

afterEach(() => {
  clearAccessToken()
})

describe('access token container', () => {
  it('无令牌时退出也会使在途冷启动刷新失效', async () => {
    let resolveRefresh
    const refreshReply = new Promise((resolve) => {
      resolveRefresh = resolve
    })
    const refreshing = refreshAccessTokenSession(() => refreshReply)
    const started = getAccessTokenSnapshot()

    clearAccessToken()
    resolveRefresh({ access_token: 'must-not-be-installed' })

    await expect(refreshing).rejects.toMatchObject({
      code: SESSION_CHANGED_DURING_REFRESH,
    })
    expect(getAccessTokenSnapshot()).toMatchObject({
      token: null,
      accessTokenGeneration: started.accessTokenGeneration + 1,
      sessionGeneration: started.sessionGeneration + 1,
      refreshAllowed: false,
      sessionActive: false,
    })
  })

  it('分别记录令牌轮换和登录会话代次', () => {
    const initial = getAccessTokenSnapshot()

    startAccessTokenSession('first-token')
    const started = getAccessTokenSnapshot()
    setAccessToken('renewed-token')
    const renewed = getAccessTokenSnapshot()
    clearAccessToken()
    const cleared = getAccessTokenSnapshot()

    expect(started).toMatchObject({
      token: 'first-token',
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
      accessTokenGeneration: renewed.accessTokenGeneration + 1,
      sessionGeneration: renewed.sessionGeneration + 1,
    })
  })

  it('快照不可变且空令牌清理仍终止当前会话', () => {
    const initial = getAccessTokenSnapshot()

    clearAccessToken()

    expect(Object.isFrozen(initial)).toBe(true)
    expect(getAccessTokenSnapshot()).toMatchObject({
      token: null,
      accessTokenGeneration: initial.accessTokenGeneration + 1,
      sessionGeneration: initial.sessionGeneration + 1,
      refreshAllowed: false,
      sessionActive: false,
    })
  })
})
