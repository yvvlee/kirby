import client, { type KirbyRequestOptions } from './client'

export type SetupStatus = {
  initialized: boolean
}

export type InitialAdministrator = {
  username: string
  display_name: string
  password: string
}

const anonymousRequest: KirbyRequestOptions = {
  skipAuthRefresh: true,
  skipAccessToken: true,
}

/**
 * Reads installation state without relying on an existing login session.
 * Parameters: none.
 * Returns: whether the initial administrator has been created.
 * Throws: HTTP failures or an invalid status response.
 */
export async function getSetupStatus(): Promise<SetupStatus> {
  const { data } = await client.get<unknown>('/setup/status', anonymousRequest)
  if (typeof data !== 'object' || data === null || !('initialized' in data)) {
    throw new TypeError('初始化状态响应无效')
  }
  if (typeof data.initialized !== 'boolean') {
    throw new TypeError('初始化状态响应无效')
  }
  return { initialized: data.initialized }
}

/**
 * Creates the administrator once before normal authentication is available.
 * Parameters: administrator contains the account and its initial password.
 * Returns: completion after the administrator has been persisted.
 * Throws: HTTP failures, including 409 when another administrator already exists.
 */
export async function createInitialAdministrator(
  administrator: InitialAdministrator,
): Promise<void> {
  await client.post('/setup/admin', administrator, anonymousRequest)
}
