import { useCallback, useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { ApiError } from '../api/errors'
import type { DayOffReason, RequestType } from '../api/types'
import { calendarMap, calendarYear, todayISO } from '../domain/dates'
import { hasReject, previewLeave } from '../domain/leave'
import { formatDay, formatDays } from '../format'
import { translateError, useI18n } from '../i18n/context'
import { useApi } from '../session'
import { useLoad } from '../useLoad'

const types: RequestType[] = ['vacation', 'day_off', 'sick']
const reasons: DayOffReason[] = [
  'unpaid_agreement',
  'unpaid_childbirth',
  'unpaid_marriage',
  'unpaid_bereavement',
  'compensatory_overtime',
  'compensatory_weekend',
]
const mustGrant = new Set<DayOffReason>(['unpaid_childbirth', 'unpaid_marriage', 'unpaid_bereavement'])

export function RequestWizard() {
  const api = useApi()
  const navigate = useNavigate()
  const { t, language } = useI18n()
  const [params] = useSearchParams()
  const today = todayISO()
  const year = params.get('year')
  const typeParam = params.get('type')
  const initialType: RequestType =
    typeParam === 'day_off' || typeParam === 'sick' || typeParam === 'vacation' ? typeParam : 'vacation'
  const planningNext = year === String(calendarYear(today) + 1) && initialType === 'vacation'
  const [type, setType] = useState<RequestType>(initialType)
  const [start, setStart] = useState(planningNext && year ? `${year}-07-01` : '')
  const [end, setEnd] = useState(planningNext && year ? `${year}-07-14` : '')
  const [reason, setReason] = useState<DayOffReason | ''>('')
  const [certificate, setCertificate] = useState('')
  const [comment, setComment] = useState('')
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  const load = useCallback(async () => {
    if (!start || !end || end < start) return null
    const [days, existing, balance] = await Promise.all([
      api.getCalendar(start, end),
      api.getRequests({ scope: 'mine' }),
      api.getBalance(start),
    ])
    return {
      balance,
      preview: previewLeave({
        type,
        start,
        end,
        reasonCode: reason || undefined,
        calendar: calendarMap(days),
        existing,
        balance,
        today,
      }),
    }
  }, [api, start, end, type, reason, today])

  const { data, error } = useLoad(load, [load])
  const preview = data?.preview
  const blocked = !preview || hasReject(preview) || (type === 'day_off' && !reason)
  const min = year ? (year === String(calendarYear(today)) ? today : `${year}-01-01`) : today
  const max = year ? `${year}-12-31` : undefined

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (blocked) return
    setPending(true)
    setSubmitError(null)
    try {
      const created = await api.createRequest({
        type,
        start,
        end,
        reasonCode: type === 'day_off' && reason ? reason : undefined,
        certificateNumber: type === 'sick' && certificate ? certificate : undefined,
        comment: comment || undefined,
      })
      navigate(`/requests/${created.id}`)
    } catch (reasonCaught) {
      setSubmitError(reasonCaught instanceof ApiError ? reasonCaught.code : 'unknown')
    } finally {
      setPending(false)
    }
  }

  const streak =
    type !== 'vacation' || !data
      ? null
      : preview && preview.countedDays >= 14
        ? t('request.wizard.streakMet')
        : data.balance.hasStreak
          ? t('request.wizard.streakExists')
          : preview && data.balance.freeDays - preview.countedDays >= 14
            ? t('request.wizard.streakLater')
            : null

  return (
    <div>
      <Link className="back" to="/">
        {t('common.back')}
      </Link>
      <p className="kicker">{t('request.wizard.kicker')}</p>
      <h1 className="display">
        <span className="display-text">
          {planningNext ? t('request.wizard.nextYearTitle') : t('request.wizard.title')}
        </span>
      </h1>
      <form className="split" onSubmit={(event) => void submit(event)}>
        <div className="card stack">
          <fieldset className="field">
            <legend>{t('request.wizard.type')}</legend>
            <div className="seg">
              {types.map((item) => (
                <label key={item}>
                  <input type="radio" name="type" checked={type === item} onChange={() => setType(item)} />
                  {t(`request.type.${item}`)}
                </label>
              ))}
            </div>
          </fieldset>
          <div className="fields-2">
            <label className="field">
              <span>{t('common.from')}</span>
              <input type="date" required min={min} max={max} value={start} onChange={(event) => setStart(event.target.value)} />
            </label>
            <label className="field">
              <span>{t('common.to')}</span>
              <input type="date" required min={start || min} max={max} value={end} onChange={(event) => setEnd(event.target.value)} />
            </label>
          </div>
          {year ? <p className="muted">{t('request.wizard.yearBound')}</p> : null}
          {type === 'day_off' ? (
            <label className="field">
              <span>{t('request.wizard.reason')}</span>
              <select required value={reason} onChange={(event) => setReason(event.target.value as DayOffReason)}>
                <option value="" />
                {reasons.map((item) => (
                  <option key={item} value={item}>
                    {t(`reason.${item}`)}
                  </option>
                ))}
              </select>
              {reason && mustGrant.has(reason) ? <span className="muted">{t('reason.mustGrantHint')}</span> : null}
            </label>
          ) : null}
          {type === 'sick' ? (
            <label className="field">
              <span>
                {t('request.wizard.certificate')} <span className="muted">({t('common.optional')})</span>
              </span>
              <input value={certificate} onChange={(event) => setCertificate(event.target.value)} />
              <span className="muted">{t('request.wizard.certificateHint')}</span>
            </label>
          ) : null}
          <label className="field">
            <span>
              {t('request.wizard.comment')} <span className="muted">({t('common.optional')})</span>
            </span>
            <textarea rows={3} value={comment} onChange={(event) => setComment(event.target.value)} />
          </label>
          {submitError ? <p className="banner banner-warn">{translateError(t, submitError)}</p> : null}
          <button className="btn btn-primary" type="submit" disabled={pending || blocked}>
            {t('request.wizard.submit')}
          </button>
        </div>
        <aside className="card preview" aria-label={t('request.wizard.preview')}>
          <h2>{t('request.wizard.preview')}</h2>
          {error ? <p className="banner banner-warn">{translateError(t, error)}</p> : null}
          {!preview ? <p className="muted">{t('request.wizard.dates')}</p> : null}
          {preview && data ? (
            <>
              <p className="preview-count">
                {t('request.wizard.counted')}
                <strong>{formatDays(language, preview.countedDays)}</strong>
              </p>
              <p>
                {t('request.wizard.holidays')}:{' '}
                {preview.holidayDates.length
                  ? preview.holidayDates.map((date) => formatDay(date, language)).join(', ')
                  : t('request.wizard.noneHolidays')}
              </p>
              <p>
                {t('request.wizard.weekends')}:{' '}
                {preview.weekendDates.length
                  ? preview.weekendDates.map((date) => formatDay(date, language)).join(', ')
                  : t('request.wizard.noneWeekends')}
              </p>
              {type === 'vacation' ? (
                <>
                  <p>
                    {t('request.wizard.payBy')}: <strong>{formatDay(preview.payBy, language)}</strong>
                  </p>
                  <p className="muted">{t('request.wizard.payByNote')}</p>
                  <p>
                    {t('request.wizard.workingYear')}: {formatDay(data.balance.workingYearStart, language)} —{' '}
                    {formatDay(data.balance.workingYearEnd, language)}
                  </p>
                  {streak ? <p>{streak}</p> : null}
                </>
              ) : null}
              {preview.issues.map((issue) => (
                <p key={issue.code} className={issue.level === 'reject' ? 'banner banner-warn' : 'banner banner-info'}>
                  {translateError(t, issue.code)}
                </p>
              ))}
            </>
          ) : null}
        </aside>
      </form>
    </div>
  )
}
