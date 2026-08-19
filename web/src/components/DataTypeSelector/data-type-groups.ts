import { baseTypeOptions } from '@/domain/schema'

type NamedResource = { key: string; name?: string }
export type DataTypeOption = { label: string; value: string }
export type DataTypeGroup = { label: string; options: readonly DataTypeOption[] }

export function buildDataTypeGroups({
  models,
  enums,
  limitedModels,
  limit,
}: {
  models: NamedResource[]
  enums: NamedResource[]
  limitedModels: NamedResource[]
  limit: boolean
}): DataTypeGroup[] {
  const groups: DataTypeGroup[] = [
    { label: '基本类型', options: baseTypeOptions },
  ]
  if (enums.length > 0) {
    groups.push({
      label: '枚举',
      options: enums.map((item) => ({
        label: item.name ?? item.key,
        value: JSON.stringify({ enumKey: item.key }),
      })),
    })
  }
  const selectableModels = limit ? limitedModels : models
  if (selectableModels.length > 0) {
    groups.push({
      label: '模型',
      options: selectableModels.map((item) => ({
        label: item.name ?? item.key,
        value: JSON.stringify({ structureKey: item.key }),
      })),
    })
  }
  return groups
}
