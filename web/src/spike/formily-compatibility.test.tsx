import { createForm } from '@formily/core'
import { connect, createSchemaField, FormProvider } from '@formily/react'
import {
  ArrayCards,
  ArrayItems,
  DatePicker,
  FormItem,
  Input,
  Space,
  TimePicker,
} from '@formily/antd-v5'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StrictMode } from 'react'
import { describe, expect, it } from 'vitest'

type FileFieldProps = {
  value?: string
  onChange?: (value: string) => void
}

const FileField = connect(({ value = '', onChange }: FileFieldProps) => (
  <div>
    <output aria-label="文件值">{value}</output>
    <button type="button" onClick={() => onChange?.('/assets/uploaded.png')}>
      选择测试文件
    </button>
  </div>
))

const SchemaField = createSchemaField({
  components: {
    ArrayCards,
    ArrayItems,
    DatePicker,
    FileField,
    FormItem,
    Input,
    Space,
    TimePicker,
  },
})

const schema = {
  type: 'object',
  properties: {
    date: {
      type: 'string',
      title: '日期',
      'x-decorator': 'FormItem',
      'x-component': 'DatePicker',
      'x-component-props': { format: 'YYYY-MM-DD' },
    },
    time: {
      type: 'string',
      title: '时间',
      'x-decorator': 'FormItem',
      'x-component': 'TimePicker',
      'x-component-props': { format: 'HH:mm:ss' },
    },
    dateRange: {
      type: 'array',
      title: '日期范围',
      'x-decorator': 'FormItem',
      'x-component': 'DatePicker.RangePicker',
      'x-component-props': { format: 'YYYY-MM-DD' },
    },
    timeRange: {
      type: 'array',
      title: '时间范围',
      'x-decorator': 'FormItem',
      'x-component': 'TimePicker.RangePicker',
      'x-component-props': { format: 'HH:mm:ss' },
    },
    file: {
      type: 'string',
      title: '文件',
      'x-decorator': 'FormItem',
      'x-component': 'FileField',
    },
    tags: {
      type: 'array',
      title: '标签',
      'x-decorator': 'FormItem',
      'x-component': 'ArrayItems',
      items: {
        type: 'void',
        'x-component': 'Space',
        properties: {
          value: {
            type: 'string',
            'x-component': 'Input',
            'x-component-props': { 'aria-label': '标签值' },
          },
          moveDown: {
            type: 'void',
            'x-component': 'ArrayItems.MoveDown',
            'x-component-props': { title: '下移标签' },
          },
          remove: {
            type: 'void',
            'x-component': 'ArrayItems.Remove',
            'x-component-props': { title: '删除标签' },
          },
        },
      },
      properties: {
        add: {
          type: 'void',
          title: '添加标签',
          'x-component': 'ArrayItems.Addition',
          'x-component-props': { defaultValue: 'new-tag' },
        },
      },
    },
    records: {
      type: 'array',
      title: '记录',
      'x-decorator': 'FormItem',
      'x-component': 'ArrayCards',
      items: {
        type: 'object',
        properties: {
          index: { type: 'void', 'x-component': 'ArrayCards.Index' },
          name: {
            type: 'string',
            title: '记录名称',
            'x-decorator': 'FormItem',
            'x-component': 'Input',
            'x-component-props': { 'aria-label': '记录名称' },
          },
          moveUp: {
            type: 'void',
            'x-component': 'ArrayCards.MoveUp',
            'x-component-props': { title: '上移记录' },
          },
          remove: {
            type: 'void',
            'x-component': 'ArrayCards.Remove',
            'x-component-props': { title: '删除记录' },
          },
        },
      },
      properties: {
        add: {
          type: 'void',
          title: '添加记录',
          'x-component': 'ArrayCards.Addition',
          'x-component-props': { defaultValue: { name: 'new-record' } },
        },
      },
    },
  },
} as const

const initialValues = {
  date: '2026-08-19',
  time: '15:30:45',
  dateRange: ['2026-08-01', '2026-08-31'],
  timeRange: ['09:00:00', '18:00:00'],
  file: '/assets/original.png',
  tags: ['alpha', 'beta'],
  records: [{ name: 'first' }, { name: 'second' }],
}

describe('React 19 Formily compatibility gate', () => {
  it('keeps date and time values as strings and writes a custom file field', async () => {
    const user = userEvent.setup()
    const form = createForm({ values: initialValues })

    render(
      <StrictMode>
        <FormProvider form={form}>
          <SchemaField schema={schema} />
        </FormProvider>
      </StrictMode>,
    )

    expect(form.values.date).toBe('2026-08-19')
    expect(form.values.time).toBe('15:30:45')
    expect(form.values.dateRange).toEqual(['2026-08-01', '2026-08-31'])
    expect(form.values.timeRange).toEqual(['09:00:00', '18:00:00'])
    expect(form.values.date).not.toHaveProperty('$d')

    await user.click(screen.getByRole('button', { name: '选择测试文件' }))

    expect(form.values.file).toBe('/assets/uploaded.png')
    expect(screen.getByLabelText('文件值')).toHaveTextContent(
      '/assets/uploaded.png',
    )
  })

  it('adds, removes, and moves primitive array values', async () => {
    const user = userEvent.setup()
    const form = createForm({ values: initialValues })

    render(
      <StrictMode>
        <FormProvider form={form}>
          <SchemaField schema={schema} />
        </FormProvider>
      </StrictMode>,
    )

    await user.click(screen.getAllByRole('button', { name: /下移标签/ })[0]!)
    expect(form.values.tags).toEqual(['beta', 'alpha'])

    await user.click(screen.getAllByRole('button', { name: /删除标签/ })[0]!)
    expect(form.values.tags).toEqual(['alpha'])

    await user.click(screen.getByRole('button', { name: /添加标签/ }))
    expect(form.values.tags).toEqual(['alpha', 'new-tag'])
  })

  it('adds, removes, and moves object array values', async () => {
    const user = userEvent.setup()
    const form = createForm({ values: initialValues })

    render(
      <StrictMode>
        <FormProvider form={form}>
          <SchemaField schema={schema} />
        </FormProvider>
      </StrictMode>,
    )

    await user.click(screen.getAllByRole('button', { name: /上移记录/ })[1]!)
    expect(form.values.records).toEqual([
      { name: 'second' },
      { name: 'first' },
    ])

    await user.click(screen.getAllByRole('button', { name: /删除记录/ })[0]!)
    expect(form.values.records).toEqual([{ name: 'first' }])

    await user.click(screen.getByRole('button', { name: /添加记录/ }))
    expect(form.values.records).toEqual([
      { name: 'first' },
      { name: 'new-record' },
    ])
  })
})
