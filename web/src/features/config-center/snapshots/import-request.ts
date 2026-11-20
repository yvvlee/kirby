import { localizedError } from '@/i18n/errors'

export type ImportRequestIdentity = { signature: string; key: string }

export function nextImportRequestIdentity(
  current: ImportRequestIdentity,
  signature: string,
  createKey: () => string,
): ImportRequestIdentity {
  if (!signature) throw localizedError('快照导入请求签名不能为空', {}, TypeError)
  if (!current.key) return { signature, key: createKey() }
  if (current.signature && current.signature !== signature) {
    return { signature, key: createKey() }
  }
  return { signature, key: current.key }
}
