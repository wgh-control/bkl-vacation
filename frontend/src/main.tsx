import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { App } from './App'
import { AuthGate } from './auth/AuthGate'
import { I18nProvider } from './i18n/context'
import './styles/global.css'
import './shell/background-logo.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <I18nProvider>
      <BrowserRouter>
        <AuthGate>
          <App />
        </AuthGate>
      </BrowserRouter>
    </I18nProvider>
  </StrictMode>,
)
