export type SchemaValueType = {
  baseType?: string
  enumKey?: string
  structureKey?: string
}

export type SchemaValueConfig = {
  key: string
  name?: string
  description?: string
  isArray?: boolean
  type: SchemaValueType
}

export type ModelField = SchemaValueConfig & {
  children?: SchemaNode[]
}

export type SchemaNode = {
  value: SchemaValueConfig
  children?: SchemaNode[]
}

export type ModelResource = {
  key: string
  fields?: ModelField[]
  children?: ModelField[]
}

export type EnumResource = {
  key: string
  values?: Array<{ label: string; value: string }>
}

export type SchemaResources = {
  models: ModelResource[]
  enums: EnumResource[]
}

export type JsonSchema = {
  type?: string
  title?: string
  description?: string
  enum?: Array<{ label: string; value: string }>
  properties?: Record<string, JsonSchema>
  items?: JsonSchema
  ['x-decorator']?: string
  ['x-decorator-props']?: Record<string, unknown>
  ['x-component']?: string
  ['x-component-props']?: Record<string, unknown>
}
