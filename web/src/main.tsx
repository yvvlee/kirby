import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'

import App from './App'
import { getLocale, t } from './i18n'
import { ApplicationProviders } from './app/providers'
import './index.css'
import './components/components.css'

document.documentElement.lang = getLocale()
document.title = t('Kirby 配置管理平台')

const rootElement = document.getElementById('root')
if (!rootElement) {
  throw new Error('React root element is missing')
}

createRoot(rootElement).render(
  <StrictMode>
    <ApplicationProviders>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </ApplicationProviders>
  </StrictMode>,
)
