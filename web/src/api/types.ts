export type Identifier = number | string

export type ApiObject = Record<string, unknown>

export type ApiEntity = ApiObject & {
  id: Identifier
}

export function isIdentifier(value: unknown): value is Identifier {
  return (typeof value === 'number' && Number.isSafeInteger(value) && value > 0)
    || (typeof value === 'string' && /^[1-9]\d*$/.test(value))
}

export type ApiListReply<T extends ApiObject = ApiEntity> = ApiObject & {
  list: T[]
  total?: number
}

export type User = ApiEntity & {
  username: string
  display_name?: string
  is_system_admin: boolean
  enabled?: boolean
  version: number
}

export type Environment = ApiEntity & {
  key: string
  name: string
  description?: string
  enabled: boolean
  version: number
}

export type PermissionReply = ApiObject & {
  permissions: string[]
}
