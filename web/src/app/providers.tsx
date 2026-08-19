import { QueryClientProvider } from '@tanstack/react-query'
import { App as AntdApp, ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import type { PropsWithChildren } from 'react'

import { createQueryClient } from './query-client'
import { AuthProvider } from '@/auth/auth-context'
import { EnvironmentProvider } from '@/auth/environment-context'

const queryClient = createQueryClient()

export function ApplicationProviders({ children }: PropsWithChildren) {
  return (
    <ConfigProvider locale={zhCN}>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <EnvironmentProvider><AntdApp>{children}</AntdApp></EnvironmentProvider>
        </AuthProvider>
      </QueryClientProvider>
    </ConfigProvider>
  )
}
