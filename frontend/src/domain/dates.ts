/** Calendar dates as YYYY-MM-DD in the local calendar, never via UTC parsing. */

export function parseISODate(iso: string): Date {
  const [year, month, day] = iso.split('-').map(Number)
  return new Date(year, (month || 1) - 1, day || 1)
}

export function formatISODate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function todayISO(now = new Date()): string {
  return formatISODate(now)
}

export function addDays(iso: string, days: number): string {
  const date = parseISODate(iso)
  date.setDate(date.getDate() + days)
  return formatISODate(date)
}

export function daysUntil(from: string, to: string): number {
  const ms = parseISODate(to).getTime() - parseISODate(from).getTime()
  return Math.round(ms / 86_400_000)
}

export function eachDate(start: string, end: string): string[] {
  if (!start || !end || end < start) return []
  const dates: string[] = []
  let cursor = start
  while (cursor <= end && dates.length < 400) {
    dates.push(cursor)
    cursor = addDays(cursor, 1)
  }
  return dates
}

export function isWeekend(iso: string): boolean {
  const day = parseISODate(iso).getDay()
  return day === 0 || day === 6
}

/** Article 112 federal non-working holidays. Weekend transfers stay in calendar data. */
const FEDERAL_MONTH_DAY = new Set([
  '01-01',
  '01-02',
  '01-03',
  '01-04',
  '01-05',
  '01-06',
  '01-07',
  '01-08',
  '02-23',
  '03-08',
  '05-01',
  '05-09',
  '06-12',
  '11-04',
])

export function federalHolidayCode(iso: string): string | null {
  const monthDay = iso.slice(5)
  return FEDERAL_MONTH_DAY.has(monthDay) ? monthDay : null
}

export function calendarYear(iso: string): number {
  return Number(iso.slice(0, 4))
}

export function planYears(hireDate: string | undefined, today: string): number[] {
  const current = calendarYear(today)
  const start = hireDate ? calendarYear(hireDate) : current - 1
  const years: number[] = []
  for (let year = start; year <= current + 1; year += 1) years.push(year)
  return years
}

export function yearKind(year: number, today: string): 'past' | 'current' | 'next' {
  const current = calendarYear(today)
  if (year < current) return 'past'
  if (year > current) return 'next'
  return 'current'
}

export function yearBounds(year: number): { from: string; to: string } {
  return { from: `${year}-01-01`, to: `${year}-12-31` }
}

export function inYear(start: string, end: string, year: number): boolean {
  const { from, to } = yearBounds(year)
  return start <= to && end >= from
}

/** Working year counted from the hire anniversary (articles 121 and 122). */
export function workingYearContaining(hireDate: string, on: string): { start: string; end: string } {
  const hire = parseISODate(hireDate)
  const point = parseISODate(on)
  let startYear = point.getFullYear()
  const anniversaryThisYear = new Date(startYear, hire.getMonth(), hire.getDate())
  if (point < anniversaryThisYear) startYear -= 1
  const start = new Date(startYear, hire.getMonth(), hire.getDate())
  const end = new Date(startYear + 1, hire.getMonth(), hire.getDate())
  end.setDate(end.getDate() - 1)
  return { start: formatISODate(start), end: formatISODate(end) }
}

export function monthsBetween(from: string, to: string): number {
  const start = parseISODate(from)
  const end = parseISODate(to)
  let months = (end.getFullYear() - start.getFullYear()) * 12 + (end.getMonth() - start.getMonth())
  if (end.getDate() < start.getDate()) months -= 1
  return months
}

export type DayKind = 'work' | 'weekend' | 'holiday' | 'transferred'

export type CountedRange = {
  countedDays: number
  calendarDays: number
  holidayDates: string[]
  weekendDates: string[]
}

export function calendarMap(days: ReadonlyArray<{ date: string; kind: DayKind }>): Map<string, DayKind> {
  return new Map(days.map((day) => [day.date, day.kind]))
}

export function countRange(
  start: string,
  end: string,
  calendar: ReadonlyMap<string, DayKind> = new Map(),
): CountedRange {
  const dates = eachDate(start, end)
  const holidayDates: string[] = []
  const weekendDates: string[] = []
  for (const date of dates) {
    const kind = calendar.get(date)
    const holiday = kind === 'holiday' || (kind === undefined && federalHolidayCode(date) !== null)
    if (holiday) {
      holidayDates.push(date)
      continue
    }
    const weekend = kind === 'weekend' || kind === 'transferred' || (kind === undefined && isWeekend(date))
    if (weekend) weekendDates.push(date)
  }
  return {
    countedDays: dates.length - holidayDates.length,
    calendarDays: dates.length,
    holidayDates,
    weekendDates,
  }
}

/** Article 136. The stricter date does not count the payment day inside the three calendar days. */
export function payByDates(start: string): { classic: string; stricter: string } {
  return { classic: addDays(start, -3), stricter: addDays(start, -4) }
}

export function rangesOverlap(aStart: string, aEnd: string, bStart: string, bEnd: string): boolean {
  return aStart <= bEnd && bStart <= aEnd
}

export function countedOverlap(
  aStart: string,
  aEnd: string,
  bStart: string,
  bEnd: string,
  calendar?: ReadonlyMap<string, DayKind>,
): number {
  const start = aStart > bStart ? aStart : bStart
  const end = aEnd < bEnd ? aEnd : bEnd
  if (end < start) return 0
  return countRange(start, end, calendar).countedDays
}
