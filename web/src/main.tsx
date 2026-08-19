import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'

import App from './App'
import { ApplicationProviders } from './app/providers'
import './index.css'
import './components/components.css'

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
