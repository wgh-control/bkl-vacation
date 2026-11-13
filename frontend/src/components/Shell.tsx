import { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import type { Language, Me, Role } from '../api/types'
import { useI18n } from '../i18n/context'
import { MeProvider, useSession } from '../session'
import { BrandMark, Stage, TrafficLights } from './WindowChrome'

export function Shell() {
  const { api, preview, logout } = useSession()
  const { t, language, setLanguage } = useI18n()
  const [me, setMe] = useState<Me | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    let cancelled = false
    api.getMe().then(
      (value) => {
        if (!cancelled) setMe(value)
      },
      () => {
        if (!cancelled) setError(true)
      },
    )
    return () => {
      cancelled = true
    }
  }, [api])

  useEffect(() => {
    if (me) setLanguage(me.language)
  }, [me, setLanguage])

  async function changeLanguage(next: Language) {
    setLanguage(next)
    const updated = await api.updateLanguage(next)
    setMe(updated)
  }

  async function toggleRole(role: Extract<Role, 'manager' | 'hr'>) {
    if (!me || !preview) return
    const roles = new Set(me.roles)
    if (roles.has(role)) roles.delete(role)
    else roles.add(role)
    preview.setRoles([...roles])
    setMe(await api.getMe())
  }

  return (
    <Stage>
      <section className="window">
        <header className="titlebar">
          <TrafficLights />
          <NavLink to="/" className="brand" aria-label="LEON">
            <BrandMark alt="" />
          </NavLink>
          {me ? (
            <nav className="nav" aria-label="LEON">
              <NavLink to="/" end data-testid="nav-employee">
                {t('nav.employee')}
              </NavLink>
              {me.roles.includes('manager') ? (
                <NavLink to="/team" data-testid="nav-manager">
                  {t('nav.manager')}
                </NavLink>
              ) : null}
              {me.roles.includes('hr') ? (
                <NavLink to="/hr" data-testid="nav-hr">
                  {t('nav.hr')}
                </NavLink>
              ) : null}
            </nav>
          ) : (
            <span className="nav" />
          )}
          <div className="tools">
            {me ? (
              <div className="identity">
                <span className="identity-name">{me.displayName}</span>
                {me.position ? <span className="identity-role">{me.position}</span> : null}
              </div>
            ) : null}
            <div className="lang" role="group" aria-label={language === 'ru' ? 'Язык' : 'Language'}>
              <button type="button" aria-pressed={language === 'ru'} onClick={() => void changeLanguage('ru')}>
                RU
              </button>
              <button type="button" aria-pressed={language === 'en'} onClick={() => void changeLanguage('en')}>
                EN
              </button>
            </div>
            <button className="btn btn-ghost btn-small" type="button" onClick={logout}>
              {t('app.signOut')}
            </button>
          </div>
        </header>
        <main className="window-body page">
          {error ? <p className="banner banner-warn">{t('error.unauthorized')}</p> : null}
          {!me && !error ? <p className="muted">{t('common.loading')}</p> : null}
          {me ? (
            <MeProvider me={me}>
              <Outlet />
            </MeProvider>
          ) : null}
        </main>
      </section>
      <footer className="footer">{t('footer.note')}</footer>
      {preview && me ? (
        <div className="preview-bar">
          <span>{t('preview.banner')}</span>
          <span className="preview-roles">
            {t('preview.roles')}
            <label>
              <input type="checkbox" checked disabled readOnly /> {t('nav.employee')}
            </label>
            <label>
              <input
                type="checkbox"
                data-testid="preview-role-manager"
                checked={me.roles.includes('manager')}
                onChange={() => void toggleRole('manager')}
              />{' '}
              {t('nav.manager')}
            </label>
            <label>
              <input
                type="checkbox"
                data-testid="preview-role-hr"
                checked={me.roles.includes('hr')}
                onChange={() => void toggleRole('hr')}
              />{' '}
              {t('nav.hr')}
            </label>
          </span>
        </div>
      ) : null}
    </Stage>
  )
}
