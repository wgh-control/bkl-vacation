/**
 * Employee client area: own planned, approved, and moved days.
 * The glass shell owns the stage, title bar, and language switch.
 * This screen uses the tokens and classes in styles/global.css.
 */
import { useCallback } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import type { LeaveRequest } from '../api/types'
import { RequestCard } from '../components/RequestCard'
import { YearCalendar } from '../components/YearCalendar'
import { calendarYear, inYear, planYears, todayISO, yearBounds, yearKind } from '../domain/dates'
import { bucketOf, confirmationState, noticeDue, type BucketName } from '../domain/leave'
import { formatDay, formatDays } from '../format'
import { translateError, useI18n } from '../i18n/context'
import { useApi, useMe } from '../session'
import { useLoad } from '../useLoad'

const buckets = ['planned', 'approved', 'moved'] as const

export function EmployeeHome() {
  const api = useApi()
  const me = useMe()
  const { t, language } = useI18n()
  const [params, setParams] = useSearchParams()
  const today = todayISO()
  const years = planYears(me.hireDate, today)
  const requested = Number(params.get('year'))
  const year = years.includes(requested) ? requested : calendarYear(today)
  const { from, to } = yearBounds(year)
  const nextYear = calendarYear(today) + 1

  const load = useCallback(async () => {
    const [balance, requests, days] = await Promise.all([
      api.getBalance(today),
      api.getRequests({ scope: 'mine' }),
      api.getCalendar(from, to),
    ])
    return { balance, requests, days }
  }, [api, today, from, to])

  const { data, error, loading, reload } = useLoad(load, [load])

  async function confirm(id: string) {
    await api.confirmRequest(id)
    reload()
  }

  async function acknowledge(id: string) {
    await api.acknowledgeRequest(id)
    reload()
  }

  const kind = yearKind(year, today)
  const requests = data?.requests ?? []
  const inThisYear = requests.filter((request) => inYear(request.start, request.end, year))
  const grouped = Object.fromEntries(buckets.map((name) => [name, [] as LeaveRequest[]])) as Record<
    BucketName,
    LeaveRequest[]
  >
  for (const request of inThisYear) {
    const bucket = bucketOf(request, requests)
    if (bucket !== 'closed') grouped[bucket].push(request)
  }

  const notices = requests.filter((request) => {
    const confirmation = confirmationState(request, today)
    const payByOpen =
      request.payByWarning &&
      (request.status === 'pending_manager' || request.status === 'pending_hr' || request.status === 'scheduled')
    return confirmation !== 'none' || noticeDue(request, today) || payByOpen
  })

  return (
    <div data-testid="employee-area">
      <p className="kicker">{t('employee.kicker')}</p>
      <h1 className="display">
        <span className="display-text">{t('employee.title')}</span>
      </h1>
      <p className="profile">
        {me.position ? <span>{me.position}</span> : null}
        {me.department ? <span>{me.department}</span> : null}
        {me.employeeNumber ? (
          <span>
            {t('employee.number')} {me.employeeNumber}
          </span>
        ) : null}
        {me.manager ? (
          <span>
            {t('employee.manager')}: {me.manager.displayName}
          </span>
        ) : null}
      </p>

      {loading ? <p className="muted">{t('common.loading')}</p> : null}
      {error ? <p className="banner banner-warn">{translateError(t, error)}</p> : null}

      {data ? (
        <>
          <section className="card balance" aria-label={t('employee.workingYear')}>
            <div className="item-top">
              <h2>{t('employee.workingYear')}</h2>
              <span className="muted">
                {formatDay(data.balance.workingYearStart, language)} — {formatDay(data.balance.workingYearEnd, language)}
              </span>
            </div>
            <p className="muted">{t('employee.workingYearNote')}</p>
            <div className="stats">
              <Stat label={t('employee.granted')} value={formatDays(language, data.balance.grantedDays + data.balance.extraDays)} />
              <Stat label={t('employee.used')} value={formatDays(language, data.balance.usedDays)} />
              <Stat label={t('employee.reserved')} value={formatDays(language, data.balance.reservedDays)} />
              <Stat label={t('employee.free')} value={formatDays(language, data.balance.freeDays)} />
              <Stat label={t('employee.carryover')} value={formatDays(language, data.balance.carryoverDays)} />
            </div>
            <p className="streak">
              {data.balance.hasStreak
                ? t('employee.streakYes')
                : data.balance.streakPossible
                  ? t('employee.streakPossible')
                  : t('employee.streakNo')}
            </p>
            {data.balance.warnings.map((code) => (
              <p key={code} className="banner banner-warn">
                {translateError(t, code)}
              </p>
            ))}
          </section>

          <div className="notices">
            {notices.map((request) => {
              const confirmation = confirmationState(request, today)
              return (
                <article key={request.id} className="banner banner-info">
                  <strong>
                    {t(`request.type.${request.type}`)} · {formatDay(request.start, language)}
                  </strong>
                  {confirmation === 'ready' ? <p>{t('notice.confirmReady')}</p> : null}
                  {confirmation === 'passed' ? <p>{t('notice.confirmPassed')}</p> : null}
                  {confirmation === 'upcoming' ? <p>{t('notice.confirmUpcoming')}</p> : null}
                  {noticeDue(request, today) ? <p>{t('notice.acknowledge')}</p> : null}
                  {request.payByWarning && (request.status === 'pending_manager' || request.status === 'pending_hr') ? (
                    <p>{t('notice.payBy')}</p>
                  ) : null}
                  <div className="actions">
                    {confirmation === 'ready' || confirmation === 'passed' ? (
                      <button className="btn btn-primary btn-small" type="button" onClick={() => void confirm(request.id)}>
                        {t('detail.confirm')}
                      </button>
                    ) : null}
                    {noticeDue(request, today) ? (
                      <button className="btn btn-yellow btn-small" type="button" onClick={() => void acknowledge(request.id)}>
                        {t('detail.acknowledge')}
                      </button>
                    ) : null}
                    <Link className="btn btn-ghost btn-small" to={`/requests/${request.id}`}>
                      {t('detail.kicker')}
                    </Link>
                  </div>
                </article>
              )
            })}
          </div>

          <div className="actions">
            <Link className="btn btn-yellow" to={`/requests/new?type=vacation&year=${nextYear}`} data-testid="plan-next-year">
              {t('employee.planNextYear')}
            </Link>
            {kind === 'past' ? (
              <p className="muted">{t('employee.pastReadOnly')}</p>
            ) : (
              <Link className="btn btn-ghost-light" to="/requests/new">
                {t('employee.newRequest')}
              </Link>
            )}
          </div>

          <section className="plan" aria-label={t('employee.planFor')}>
            <div className="item-top">
              <h2 className="on-dark-title">
                {t('employee.planFor')} {year}
              </h2>
              <span className="muted">{t(`employee.${yearKind(year, today)}`)}</span>
            </div>
            <div className="years" data-testid="year-switcher">
              {years.map((item) => (
                <button
                  key={item}
                  type="button"
                  aria-pressed={item === year}
                  onClick={() => setParams({ year: String(item) })}
                >
                  <span>{item}</span>
                  <small>{t(`employee.${yearKind(item, today)}`)}</small>
                </button>
              ))}
            </div>
            <YearCalendar year={year} days={data.days} requests={inThisYear} />
            <div className="grid-3">
              {buckets.map((bucket) => (
                <section key={bucket} className={`card bucket bucket-${bucket}`} data-testid={`bucket-${bucket}`}>
                  <h3>{t(`employee.${bucket}`)}</h3>
                  {bucket === 'moved' ? <p className="muted">{t('employee.movedNote')}</p> : null}
                  {grouped[bucket].length === 0 ? <p className="empty">{t('employee.noDays')}</p> : null}
                  <div className="stack">
                    {grouped[bucket].map((request) => (
                      <RequestCard key={request.id} request={request} all={requests} />
                    ))}
                  </div>
                </section>
              ))}
            </div>
          </section>
        </>
      ) : null}
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}
