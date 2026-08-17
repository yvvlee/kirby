<template>
  <FormProvider :form="form">
    <SchemaField
      :key="formKey"
      :schema="schema"
      :components="schemaComponents"
    />
  </FormProvider>
</template>

<script>
import { createForm } from '@formily/core'
import { createSchemaField, FormProvider } from '@formily/vue'

import {
  createWrappedSchema,
  normalizeData,
  unwrapValue,
  wrapValue,
} from '@/utils/schema/index.js'
import {
  ArrayCards,
  ArrayItems,
  DatePicker,
  FormItem,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
  TimePicker,
} from './formily-components.js'

const SchemaFields = createSchemaField({
  components: {
    ArrayCards,
    ArrayItems,
    DatePicker,
    FormItem,
    Input,
    InputNumber,
    Select,
    Space,
    Switch,
    TimePicker,
  },
})

const MissingFileUpload = {
  name: 'MissingFileUpload',
  created() {
    throw new Error('文件字段需要通过 fileUploadComponent 显式注入上传组件')
  },
  render(createElement) {
    return createElement('span')
  },
}

export default {
  name: 'SchemaForm',
  components: {
    FormProvider,
    ...SchemaFields,
  },
  props: {
    config: {
      type: Object,
      default: () => ({}),
    },
    value: {
      type: String,
      default: '',
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    models: {
      type: Array,
      default: () => [],
    },
    enums: {
      type: Array,
      default: () => [],
    },
    fileUploadComponent: {
      type: [Object, Function],
      default: null,
    },
  },
  data() {
    return {
      form: createForm(),
      formKey: 0,
      schema: {},
    }
  },
  computed: {
    resources() {
      return { models: this.models, enums: this.enums }
    },
    schemaComponents() {
      return {
        FileUpload: this.fileUploadComponent || MissingFileUpload,
      }
    },
  },
  watch: {
    config: {
      deep: true,
      immediate: true,
      handler() {
        this.rebuildSchema()
      },
    },
    models: {
      deep: true,
      handler() {
        this.rebuildSchema()
      },
    },
    enums: {
      deep: true,
      handler() {
        this.rebuildSchema()
      },
    },
    value: {
      immediate: true,
      handler(value) {
        this.setValue(value)
      },
    },
    disabled: {
      immediate: true,
      handler(disabled) {
        this.form.setPattern(disabled ? 'disabled' : 'editable')
      },
    },
  },
  methods: {
    rebuildSchema() {
      if (!this.config?.value) {
        this.schema = {}
        return
      }
      this.schema = createWrappedSchema(this.config, this.resources)
      this.form = createForm({ pattern: this.disabled ? 'disabled' : 'editable' })
      this.formKey += 1
      this.setValue(this.value)
    },
    getValue() {
      if (!this.config?.value) {
        return ''
      }
      const formValue = this.form.getFormState().values
      return unwrapValue(formValue, this.config.value)
    },
    setValue(value) {
      if (!this.config?.value) {
        return
      }
      const normalizedValue = normalizeData(this.config, value, this.resources)
      const wrappedValue = wrapValue(normalizedValue, this.config.value)
      this.form.setValues(wrappedValue, 'overwrite')
    },
  },
}
</script>
