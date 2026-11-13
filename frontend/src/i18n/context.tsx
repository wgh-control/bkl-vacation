import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import type { Language } from '../api/types'
import { en } from './en'
import { ru, type MessageKey } from './ru'

const catalogs = { ru, en }

type I18nValue = {
  language: Language
  setLanguage: (language: Language) => void
  t: (key: MessageKey) => string
}

const I18nContext = createContext<I18nValue | null>(null)

export function isMessageKey(key: string): key is MessageKey {
  return Object.prototype.hasOwnProperty.call(ru, key)
}

export function translateError(t: (key: MessageKey) => string, code: string): string {
  const key = `error.${code}`
  return isMessageKey(key) ? t(key) : t('error.unknown')
}

function storedLanguage(): Language {
  try {
    return localStorage.getItem('bkl.language') === 'en' ? 'en' : 'ru'
  } catch {
    return 'ru'
  }
}

export function I18nProvider({ initial, children }: { initial?: Language; children: ReactNode }) {
  const [language, setLanguageState] = useState<Language>(initial ?? storedLanguage)
  const setLanguage = (next: Language) => {
    setLanguageState(next)
    try {
      localStorage.setItem('bkl.language', next)
    } catch {
      /* The profile update still keeps the choice for this visit. */
    }
  }
  useEffect(() => {
    document.documentElement.lang = language
    document.title = language === 'ru' ? 'Отпуска — LEON' : 'Leave — LEON'
  }, [language])
  const value = useMemo<I18nValue>(
    () => ({ language, setLanguage, t: (key) => catalogs[language][key] }),
    [language],
  )
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nValue {
  const value = useContext(I18nContext)
  if (!value) throw new Error('i18n')
  return value
}
