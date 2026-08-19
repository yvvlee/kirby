import client from './client'
import { isIdentifier, type ApiObject, type Identifier } from './types'

export function positiveId(value: Identifier, name: string): string {
  if (!isIdentifier(value)) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return String(value)
}

export function positiveIdentifier(
  value: Identifier,
  name: string,
): Identifier {
  positiveId(value, name)
  return value
}

export async function postEnvironmentOperation<T>(
  environmentId: Identifier,
  resource: string,
  operation: string,
  value: ApiObject = {},
): Promise<T> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError(`${resource} request must be an object`)
  }
  const id = positiveId(environmentId, 'environmentId')
  const { data } = await client.post<T>(
    `/admin/environments/${id}/${resource}/${operation}`,
    { ...value, environment_id: environmentId },
  )
  return data
}
