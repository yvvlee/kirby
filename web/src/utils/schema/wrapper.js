const needsWrapper = (valueConfig) => {
  if (!valueConfig?.key || !valueConfig.type) {
    throw new TypeError('配置项缺少 key 或 type')
  }
  return Boolean(
    valueConfig.type.baseType || valueConfig.type.enumKey || valueConfig.isArray,
  )
}

export const wrapSchema = (schema, valueConfig) => {
  if (needsWrapper(valueConfig)) {
    return { type: 'object', properties: schema }
  }
  return schema[valueConfig.key]
}

export const wrapValue = (value, valueConfig) =>
  needsWrapper(valueConfig) ? { [valueConfig.key]: value } : value

export const unwrapValue = (value, valueConfig) => {
  if (!needsWrapper(valueConfig)) {
    return value
  }
  if (!value || typeof value !== 'object') {
    throw new TypeError('表单值不是可解包的对象')
  }
  return value[valueConfig.key]
}
