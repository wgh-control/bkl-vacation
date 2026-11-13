import type { CalendarDay, LeaveRequest } from '../api/types'
import { formatISODate } from '../domain/dates'
import { formatMonth, weekdayLabels } from '../format'
import { isMessageKey, useI18n } from '../i18n/context'

function monthCells(year: number, month: number): Array<string | null> {
  const first = new Date(year, month, 1)
  const pad = (first.getDay() + 6) % 7
  const count = new Date(year, month + 1, 0).getDate()
  const cells: Array<string | null> = Array.from({ length: pad }, () => null)
  for (let day = 1; day <= count; day += 1) cells.push(formatISODate(new Date(year, month, day)))
  return cells
}

export function YearCalendar({
  year,
  days,
  requests,
}: {
  year: number
  days: CalendarDay[]
  requests: LeaveRequest[]
}) {
  const { t, language } = useI18n()
  const kinds = new Map(days.map((day) => [day.date, day]))
  const labels = weekdayLabels(language)
  return (
    <div>
      <div className="months">
        {Array.from({ length: 12 }, (_, month) => (
          <section className="month" key={month} aria-label={formatMonth(year, month, language)}>
            <h3>{formatMonth(year, month, language)}</h3>
            <div className="dow">
              {labels.map((label, index) => (
                <span key={`${month}-${index}`}>{label}</span>
              ))}
            </div>
            <div className="days">
              {monthCells(year, month).map((date, index) => {
                if (!date) return <span key={`empty-${index}`} />
                const kind = kinds.get(date)?.kind
                const covering = requests.filter(
                  (request) =>
                    request.start <= date &&
                    request.end >= date &&
                    request.status !== 'rejected' &&
                    request.status !== 'cancelled' &&
                    request.status !== 'draft',
                )
                const type = (['sick', 'vacation', 'day_off'] as const).find((item) =>
                  covering.some((request) => request.type === item),
                )
                const holidayName = kinds.get(date)?.name
                const holidayKey = holidayName ? `holiday.${holidayName}` : ''
                const title = [
                  date,
                  type ? t(`request.type.${type}`) : '',
                  holidayKey && isMessageKey(holidayKey) ? t(holidayKey) : holidayName,
                ]
                  .filter(Boolean)
                  .join(' · ')
                return (
                  <span
                    key={date}
                    className={['day', kind ?? '', type ? `has-${type}` : ''].filter(Boolean).join(' ')}
                    title={title}
                  >
                    {Number(date.slice(8))}
                  </span>
                )
              })}
            </div>
          </section>
        ))}
      </div>
      <ul className="legend">
        <li><i className="swatch has-vacation" /> {t('calendar.vacation')}</li>
        <li><i className="swatch has-day_off" /> {t('calendar.dayOff')}</li>
        <li><i className="swatch has-sick" /> {t('calendar.sick')}</li>
        <li><i className="swatch holiday" /> {t('calendar.holiday')}</li>
        <li><i className="swatch weekend" /> {t('calendar.weekend')}</li>
      </ul>
    </div>
  )
}
