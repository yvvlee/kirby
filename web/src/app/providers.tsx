import { QueryClientProvider } from '@tanstack/react-query'
import { App as AntdApp, ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import { useTranslation } from '@/i18n'
import type { PropsWithChildren } from 'react'

import { createQueryClient } from './query-client'
import { AuthProvider } from '@/auth/auth-context'
import { EnvironmentProvider } from '@/auth/environment-context'

const queryClient = createQueryClient()

/**
 * Applies one locale to framework controls and application content.
 * Parameters: children contains the application routes.
 * Returns: shared application providers without remounting on locale changes.
 * Throws: provider initialization errors.
 */
export function ApplicationProviders({ children }: PropsWithChildren) {
  const { locale } = useTranslation()
  return (
    <ConfigProvider locale={locale === 'zh-CN' ? zhCN : enUS}>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <EnvironmentProvider><AntdApp>{children}</AntdApp></EnvironmentProvider>
        </AuthProvider>
      </QueryClientProvider>
    </ConfigProvider>
  )
}
