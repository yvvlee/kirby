export type ImportRequestIdentity = { signature: string; key: string }

export function nextImportRequestIdentity(
  current: ImportRequestIdentity,
  signature: string,
  createKey: () => string,
): ImportRequestIdentity {
  if (!signature) throw new TypeError('快照导入请求签名不能为空')
  if (!current.key) return { signature, key: createKey() }
  if (current.signature && current.signature !== signature) {
    return { signature, key: createKey() }
  }
  return { signature, key: current.key }
}
