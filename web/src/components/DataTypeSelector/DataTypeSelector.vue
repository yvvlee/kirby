<template>
  <el-select
    :value="value"
    :placeholder="placeholder"
    size="small"
    @change="$emit('input', $event)"
  >
    <el-option-group
      v-for="group in options"
      :key="group.label"
      :label="group.label"
    >
      <el-option
        v-for="item in group.options"
        :key="item.value"
        :label="item.label"
        :value="item.value"
      />
    </el-option-group>
  </el-select>
</template>

<script>
import { baseTypeOptions } from '@/utils/schema/index.js'

export const buildDataTypeGroups = ({ models, enums, limitedModels, limit }) => {
  const groups = [
    {
      label: '基本类型',
      options: baseTypeOptions,
    },
  ]

  if (enums.length > 0) {
    groups.push({
      label: '枚举',
      options: enums.map((item) => ({
        label: item.name,
        value: JSON.stringify({ enumKey: item.key }),
      })),
    })
  }

  const selectableModels = limit ? limitedModels : models
  if (selectableModels.length > 0) {
    groups.push({
      label: '模型',
      options: selectableModels.map((item) => ({
        label: item.name,
        value: JSON.stringify({ structureKey: item.key }),
      })),
    })
  }
  return groups
}

export default {
  name: 'DataTypeSelector',
  props: {
    value: {
      type: String,
      default: '',
    },
    models: {
      type: Array,
      default: () => [],
    },
    enums: {
      type: Array,
      default: () => [],
    },
    limitedModels: {
      type: Array,
      default: () => [],
    },
    limit: {
      type: Boolean,
      default: false,
    },
    optionGroups: {
      type: Array,
      default: () => [],
    },
    placeholder: {
      type: String,
      default: '请选择数据类型',
    },
  },
  computed: {
    options() {
      if (this.optionGroups.length > 0) {
        return this.optionGroups
      }
      return buildDataTypeGroups({
        models: this.models,
        enums: this.enums,
        limitedModels: this.limitedModels,
        limit: this.limit,
      })
    },
  },
}
</script>
