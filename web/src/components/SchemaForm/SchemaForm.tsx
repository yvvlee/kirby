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
import { t, useTranslation } from '@/i18n'

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
  throw new Error(t('文件字段需要通过 fileUploadComponent 显式注入上传组件'))
}

// Stable empty resources prevent language-only renders from reapplying saved values.
const emptyModels: ModelResource[] = []
const emptyEnums: EnumResource[] = []

const SchemaForm = forwardRef<SchemaFormHandle, SchemaFormProps>(
  function SchemaForm(
    {
      config,
      value = '',
      disabled = false,
      models = emptyModels,
      enums = emptyEnums,
      fileUploadComponent = MissingFileUpload,
    },
    ref,
  ) {
    const { locale } = useTranslation()
    const resources = useMemo(() => ({ models, enums }), [enums, models])
    const schema = useMemo(
      () => config ? createWrappedSchema(config, resources) : {},
      // createWrappedSchema reads the active locale through the shared translator.
      // eslint-disable-next-line react-hooks/exhaustive-deps
      [config, locale, resources],
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
