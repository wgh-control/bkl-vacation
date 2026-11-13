import type { Language } from './api/types'
import { parseISODate } from './domain/dates'

const locale = (language: Language) => (language === 'ru' ? 'ru-RU' : 'en-GB')

export function formatDay(iso: string, language: Language): string {
  return new Intl.DateTimeFormat(locale(language), {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }).format(parseISODate(iso))
}

export function formatRange(start: string, end: string, language: Language): string {
  if (start === end) return formatDay(start, language)
  return `${formatDay(start, language)} — ${formatDay(end, language)}`
}

export function formatMonth(year: number, month: number, language: Language): string {
  return new Intl.DateTimeFormat(locale(language), { month: 'long' }).format(new Date(year, month, 1))
}

export function weekdayLabels(language: Language): string[] {
  const formatter = new Intl.DateTimeFormat(locale(language), { weekday: 'short' })
  return [0, 1, 2, 3, 4, 5, 6].map((offset) => formatter.format(new Date(2026, 0, 5 + offset)))
}

export function formatDays(language: Language, count: number): string {
  if (language === 'en') return `${count} ${count === 1 ? 'day' : 'days'}`
  const abs = Math.abs(count) % 100
  const last = abs % 10
  if (abs > 10 && abs < 20) return `${count} дней`
  if (last === 1) return `${count} день`
  if (last >= 2 && last <= 4) return `${count} дня`
  return `${count} дней`
}
