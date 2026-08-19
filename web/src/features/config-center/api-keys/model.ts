import type { ApiKeySecretReply, ProjectApiKey } from '@/api/api-keys'

export function requireKeyList(reply: unknown): ProjectApiKey[] {
  if (typeof reply !== 'object' || reply === null || !('list' in reply) || !Array.isArray(reply.list)) {
    throw new TypeError('API Key 列表响应缺少 list')
  }
  reply.list.forEach((item) => {
    if (typeof item !== 'object' || item === null || Array.isArray(item)) throw new TypeError('API Key 列表项必须是对象')
    if (Object.hasOwn(item, 'secret')) throw new Error('API Key 列表不得包含完整 Secret')
  })
  return reply.list as ProjectApiKey[]
}

export function requireSecretReply(reply: ApiKeySecretReply): string {
  if (!reply.apiKey || typeof reply.secret !== 'string' || !reply.secret) {
    throw new TypeError('API Key 响应缺少一次性 Secret')
  }
  return reply.secret
}
