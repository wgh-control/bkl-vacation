import { useEffect, useState, type ReactNode } from 'react'
import Keycloak from 'keycloak-js'
import { createHttpApi } from '../api/http'
import type { Language } from '../api/types'
import { BrandMark, Stage, TrafficLights } from '../components/WindowChrome'
import { useI18n } from '../i18n/context'
import { SessionProvider, type Session } from '../session'

function keycloakConfigured(): boolean {
  return Boolean(
    import.meta.env.VITE_KEYCLOAK_URL &&
      import.meta.env.VITE_KEYCLOAK_REALM &&
      import.meta.env.VITE_KEYCLOAK_CLIENT_ID,
  )
}

export function AuthGate({ children }: { children: ReactNode }) {
  const { t, language, setLanguage } = useI18n()
  const [phase, setPhase] = useState<'loading' | 'config' | 'error' | 'ready'>('loading')
  const [session, setSession] = useState<Session | null>(null)

  useEffect(() => {
    let cancelled = false
    async function start() {
      if (import.meta.env.DEV && import.meta.env.VITE_PREVIEW === '1') {
        const { createFixtureApi } = await import('../api/fixture')
        const api = createFixtureApi()
        if (cancelled) return
        setSession({
          api,
          preview: { setRoles: (roles) => api.setRoles(roles) },
          logout: () => window.location.assign('/'),
        })
        setPhase('ready')
        return
      }
      if (!keycloakConfigured()) {
        if (!cancelled) setPhase('config')
        return
      }
      try {
        const keycloak = new Keycloak({
          url: import.meta.env.VITE_KEYCLOAK_URL ?? '',
          realm: import.meta.env.VITE_KEYCLOAK_REALM ?? '',
          clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID ?? '',
        })
        await keycloak.init({
          pkceMethod: 'S256',
          onLoad: 'login-required',
          checkLoginIframe: false,
          flow: 'standard',
        })
        if (cancelled) return
        setSession({
          api: createHttpApi({
            token: async () => {
              await keycloak.updateToken(30)
              if (!keycloak.token) throw new Error('unauthorized')
              return keycloak.token
            },
          }),
          preview: null,
          logout: () => {
            void keycloak.logout({ redirectUri: window.location.origin })
          },
        })
        setPhase('ready')
      } catch {
        if (!cancelled) setPhase('error')
      }
    }
    void start()
    return () => {
      cancelled = true
    }
  }, [])

  if (phase === 'ready' && session) return <SessionProvider value={session}>{children}</SessionProvider>

  return (
    <Stage>
      <section className="window gate-window">
        <header className="titlebar">
          <TrafficLights />
          <BrandMark />
          <div className="tools">
            <LanguageSwitch language={language} onChange={setLanguage} />
          </div>
        </header>
        <div className="window-body">
          {phase === 'config' ? (
            <section className="card gate-card">
              <h1>{t('config.title')}</h1>
              <p>{t('config.body')}</p>
            </section>
          ) : null}
          {phase === 'error' ? (
            <section className="card gate-card">
              <h1>{t('auth.failed')}</h1>
              <button className="btn btn-primary" type="button" onClick={() => window.location.reload()}>
                {t('auth.signIn')}
              </button>
            </section>
          ) : null}
          {phase === 'loading' ? <p className="muted">{t('auth.loading')}</p> : null}
        </div>
      </section>
    </Stage>
  )
}

function LanguageSwitch({ language, onChange }: { language: Language; onChange: (language: Language) => void }) {
  return (
    <div className="lang" role="group" aria-label={language === 'ru' ? 'Язык' : 'Language'}>
      <button type="button" aria-pressed={language === 'ru'} onClick={() => onChange('ru')}>
        RU
      </button>
      <button type="button" aria-pressed={language === 'en'} onClick={() => onChange('en')}>
        EN
      </button>
    </div>
  )
}
