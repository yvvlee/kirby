import { createForm, type Form } from '@formily/core'
import type { ISchema } from '@formily/json-schema'
import { createSchemaField, FormProvider } from '@formily/react'
import {
  ArrayCards,
  ArrayItems,
  DatePicker,
  FormItem,
  Input,
  NumberPicker,
  Select,
  Space,
  Switch,
  TimePicker,
} from '@formily/antd-v5'
import {
  type ComponentType,
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
} from 'react'

import {
  createWrappedSchema,
  normalizeData,
  unwrapValue,
  wrapValue,
  type EnumResource,
  type ModelResource,
  type SchemaNode,
} from '@/domain/schema'

type FileFieldProps = {
  value?: string | string[]
  disabled?: boolean
  onChange?: (value: string | string[]) => void
  uploadType?: 'Image' | 'Video' | 'File'
  isArray?: boolean
}

export type SchemaFormHandle = {
  getValue: () => unknown
  setValue: (value: string) => void
  form: Form
}

export type SchemaFormProps = {
  config?: SchemaNode
  value?: string
  disabled?: boolean
  models?: ModelResource[]
  enums?: EnumResource[]
  fileUploadComponent?: ComponentType<FileFieldProps>
}

function MissingFileUpload(): never {
  throw new Error('文件字段需要通过 fileUploadComponent 显式注入上传组件')
}

const SchemaForm = forwardRef<SchemaFormHandle, SchemaFormProps>(
  function SchemaForm(
    {
      config,
      value = '',
      disabled = false,
      models = [],
      enums = [],
      fileUploadComponent = MissingFileUpload,
    },
    ref,
  ) {
    const resources = useMemo(() => ({ models, enums }), [enums, models])
    const schema = useMemo(
      () => config ? createWrappedSchema(config, resources) : {},
      [config, resources],
    )
    const form = useMemo(() => createForm(), [])
    const SchemaField = useMemo(
      () => createSchemaField({
        components: {
          ArrayCards,
          ArrayItems,
          DatePicker,
          FileUpload: fileUploadComponent,
          FormItem,
          Input,
          NumberPicker,
          Select,
          Space,
          Switch,
          TimePicker,
        },
      }),
      [fileUploadComponent],
    )

    const setValue = useCallback((serializedValue: string) => {
      if (!config) return
      const normalized = normalizeData(config, serializedValue, resources)
      form.setValues(wrapValue(normalized, config.value), 'overwrite')
    }, [config, form, resources])

    useEffect(() => {
      setValue(value)
    }, [setValue, value])

    useEffect(() => {
      form.setPattern(disabled ? 'disabled' : 'editable')
    }, [disabled, form])

    useImperativeHandle(ref, () => ({
      form,
      getValue: () => config
        ? unwrapValue(form.getFormState().values, config.value)
        : '',
      setValue,
    }), [config, form, setValue])

    if (!config) return null
    return (
      <FormProvider form={form}>
        <SchemaField schema={schema as ISchema} />
      </FormProvider>
    )
  },
)

export default SchemaForm
