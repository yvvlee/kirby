import type { JsonSchema, SchemaValueConfig } from './types'

function needsWrapper(valueConfig: SchemaValueConfig): boolean {
  if (!valueConfig.key || !valueConfig.type) {
    throw new TypeError('配置项缺少 key 或 type')
  }
  return Boolean(
    valueConfig.type.baseType ||
      valueConfig.type.enumKey ||
      valueConfig.isArray,
  )
}

export function wrapSchema(
  schema: Record<string, JsonSchema>,
  valueConfig: SchemaValueConfig,
): JsonSchema {
  if (needsWrapper(valueConfig)) {
    return { type: 'object', properties: schema }
  }
  const unwrapped = schema[valueConfig.key]
  if (!unwrapped) {
    throw new Error(`Schema 缺少根属性: ${valueConfig.key}`)
  }
  return unwrapped
}

export function wrapValue(
  value: unknown,
  valueConfig: SchemaValueConfig,
): unknown {
  return needsWrapper(valueConfig) ? { [valueConfig.key]: value } : value
}

export function unwrapValue(
  value: unknown,
  valueConfig: SchemaValueConfig,
): unknown {
  if (!needsWrapper(valueConfig)) {
    return value
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new TypeError('表单值不是可解包的对象')
  }
  return (value as Record<string, unknown>)[valueConfig.key]
}
