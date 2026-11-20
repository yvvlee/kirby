import { useMemo, useState } from 'react'
import { Button, Space, Typography } from 'antd'
import { createForm } from '@formily/core'
import { createSchemaField, FormProvider } from '@formily/react'
import { FormItem, Input, NumberPicker, Select, Switch } from '@formily/antd-v5'

import '@/App.css'
import { useTranslation } from '@/i18n'

const { Title, Paragraph } = Typography

export default function FormilySmokePage() {
  const { t } = useTranslation()
  const form = useMemo(() => createForm(), [])
  const [snapshot, setSnapshot] = useState('')
  const SchemaField = useMemo(
    () =>
      createSchemaField({
        components: { FormItem, Input, NumberPicker, Select, Switch },
      }),
    [],
  )

  return (
    <main className="smoke-page">
      <section className="smoke-panel" aria-labelledby="page-title">
        <Title id="page-title" level={2}>{t('Kirby React 迁移验证')}</Title>
        <Paragraph>{t('这是 R-01 的最小 Formily React 验证页面。')}</Paragraph>
        <FormProvider form={form}>
          <SchemaField
            schema={{
              type: 'object',
              properties: {
                name: {
                  type: 'string',
                  title: t('名称'),
                  required: true,
                  'x-decorator': 'FormItem',
                  'x-component': 'Input',
                  'x-component-props': { 'aria-label': t('名称') },
                },
                count: {
                  type: 'number',
                  title: t('数量'),
                  'x-decorator': 'FormItem',
                  'x-component': 'NumberPicker',
                  'x-component-props': { 'aria-label': t('数量') },
                },
                enabled: {
                  type: 'boolean',
                  title: t('启用'),
                  'x-decorator': 'FormItem',
                  'x-component': 'Switch',
                  'x-component-props': { 'aria-label': t('启用') },
                },
                mode: {
                  type: 'string',
                  title: t('模式'),
                  enum: [
                    { label: t('开发'), value: 'development' },
                    { label: t('生产'), value: 'production' },
                  ],
                  'x-decorator': 'FormItem',
                  'x-component': 'Select',
                  'x-component-props': { 'aria-label': t('模式') },
                },
              },
            }}
          />
        </FormProvider>
        <Space>
          <Button type="primary" onClick={() => setSnapshot(JSON.stringify(form.values))}>
            {t('读取表单值')}
          </Button>
          <Button onClick={() => form.setPattern('disabled')}>{t('设为只读')}</Button>
        </Space>
        {snapshot ? <pre className="form-snapshot" aria-label={t('表单值')}>{snapshot}</pre> : null}
      </section>
    </main>
  )
}
