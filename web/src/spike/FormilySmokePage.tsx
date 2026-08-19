import { useMemo, useState } from 'react'
import { Button, Space, Typography } from 'antd'
import { createForm } from '@formily/core'
import { createSchemaField, FormProvider } from '@formily/react'
import { FormItem, Input, NumberPicker, Select, Switch } from '@formily/antd-v5'

import '@/App.css'

const { Title, Paragraph } = Typography

export default function FormilySmokePage() {
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
        <Title id="page-title" level={2}>Kirby React 迁移验证</Title>
        <Paragraph>这是 R-01 的最小 Formily React 验证页面。</Paragraph>
        <FormProvider form={form}>
          <SchemaField
            schema={{
              type: 'object',
              properties: {
                name: {
                  type: 'string',
                  title: '名称',
                  required: true,
                  'x-decorator': 'FormItem',
                  'x-component': 'Input',
                  'x-component-props': { 'aria-label': '名称' },
                },
                count: {
                  type: 'number',
                  title: '数量',
                  'x-decorator': 'FormItem',
                  'x-component': 'NumberPicker',
                  'x-component-props': { 'aria-label': '数量' },
                },
                enabled: {
                  type: 'boolean',
                  title: '启用',
                  'x-decorator': 'FormItem',
                  'x-component': 'Switch',
                  'x-component-props': { 'aria-label': '启用' },
                },
                mode: {
                  type: 'string',
                  title: '模式',
                  enum: [
                    { label: '开发', value: 'development' },
                    { label: '生产', value: 'production' },
                  ],
                  'x-decorator': 'FormItem',
                  'x-component': 'Select',
                  'x-component-props': { 'aria-label': '模式' },
                },
              },
            }}
          />
        </FormProvider>
        <Space>
          <Button type="primary" onClick={() => setSnapshot(JSON.stringify(form.values))}>
            读取表单值
          </Button>
          <Button onClick={() => form.setPattern('disabled')}>设为只读</Button>
        </Space>
        {snapshot ? <pre className="form-snapshot" aria-label="表单值">{snapshot}</pre> : null}
      </section>
    </main>
  )
}
