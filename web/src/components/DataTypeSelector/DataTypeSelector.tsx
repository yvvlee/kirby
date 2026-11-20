import { Select } from 'antd'
import { useTranslation } from '@/i18n'

import {
  buildDataTypeGroups,
  type DataTypeGroup,
} from './data-type-groups'

type NamedResource = { key: string; name?: string }
type Props = {
  value?: string
  models?: NamedResource[]
  enums?: NamedResource[]
  limitedModels?: NamedResource[]
  limit?: boolean
  optionGroups?: DataTypeGroup[]
  placeholder?: string
  onChange?: (value: string) => void
}

export default function DataTypeSelector({
  value,
  models = [],
  enums = [],
  limitedModels = [],
  limit = false,
  optionGroups = [],
  placeholder,
  onChange,
}: Props) {
  const { t } = useTranslation()
  const options = optionGroups.length
    ? optionGroups
    : buildDataTypeGroups({ models, enums, limitedModels, limit })
  return (
    <Select
      value={value ?? null}
      placeholder={t(placeholder ?? '请选择数据类型')}
      options={options}
      onChange={(nextValue) => onChange?.(nextValue)}
    />
  )
}
