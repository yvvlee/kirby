import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import FormilySmokePage from './spike/FormilySmokePage'
import { renderWithProviders } from './test/render'

describe('React Formily smoke application', () => {
  it('writes and reads Formily field values', async () => {
    const user = userEvent.setup()
    renderWithProviders(<FormilySmokePage />)

    expect(
      screen.getByRole('heading', { name: 'Kirby React 迁移验证' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: '名称' })).toBeInTheDocument()
    expect(screen.getByRole('spinbutton', { name: '数量' })).toBeInTheDocument()

    await user.type(screen.getByRole('textbox', { name: '名称' }), 'kirby')
    await user.type(screen.getByRole('spinbutton', { name: '数量' }), '3')
    await user.click(screen.getByRole('button', { name: '读取表单值' }))

    expect(screen.getByLabelText('表单值')).toHaveTextContent(
      '{"name":"kirby","count":3}',
    )
  })

  it('applies disabled pattern to Formily fields', async () => {
    const user = userEvent.setup()
    renderWithProviders(<FormilySmokePage />)

    await user.click(screen.getByRole('button', { name: '设为只读' }))

    expect(screen.getByRole('textbox', { name: '名称' })).toBeDisabled()
    expect(screen.getByRole('spinbutton', { name: '数量' })).toBeDisabled()
  })
})
