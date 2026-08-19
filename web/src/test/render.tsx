import { QueryClientProvider } from '@tanstack/react-query'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { StrictMode, type ReactElement } from 'react'
import { MemoryRouter } from 'react-router-dom'
import {
  render,
  type RenderOptions,
  type RenderResult,
} from '@testing-library/react'

import { createQueryClient } from '@/app/query-client'

type ProviderRenderOptions = Omit<RenderOptions, 'wrapper'> & {
  route?: string
}

export function renderWithProviders(
  element: ReactElement,
  { route = '/', ...options }: ProviderRenderOptions = {},
): RenderResult {
  const queryClient = createQueryClient()

  return render(
    <StrictMode>
      <ConfigProvider locale={zhCN}>
        <QueryClientProvider client={queryClient}>
          <MemoryRouter initialEntries={[route]}>{element}</MemoryRouter>
        </QueryClientProvider>
      </ConfigProvider>
    </StrictMode>,
    options,
  )
}
