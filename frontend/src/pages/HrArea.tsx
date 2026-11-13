import { useCallback, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../api/errors'
import type { RequestType } from '../api/types'
import { DecisionCard } from '../components/DecisionCard'
import { YearCalendar } from '../components/YearCalendar'
import { calendarYear, todayISO, yearBounds } from '../domain/dates'
import { formatRange } from '../format'
import { translateError, useI18n } from '../i18n/context'
import { useApi } from '../session'
import { useLoad } from '../useLoad'

const filters: Array<RequestType | 'all'> = ['all', 'vacation', 'day_off', 'sick']

export function HrArea() {
  const api = useApi()
  const { t, language } = useI18n()
  const today = todayISO()
  const year = calendarYear(today)
  const { from, to } = yearBounds(year)
  const [filter, setFilter] = useState<RequestType | 'all'>('all')
  const [departmentId, setDepartmentId] = useState('all')
  const load = useCallback(async () => {
    const [queue, requests, days, departments] = await Promise.all([
      api.getHrQueue(),
      api.getRequests({ scope: 'all', from, to }),
      api.getCalendar(from, to),
      api.listDepartments(),
    ])
    return { queue, requests, days, departments }
  }, [api, from, to])
  const { data, error, loading, reload } = useLoad(load, [load])
  const pending = (data?.queue.pendingApproval ?? []).filter((request) => filter === 'all' || request.type === filter)
  const schedule = (data?.requests ?? []).filter((request) => {
    if (request.status === 'rejected' || request.status === 'cancelled' || request.status === 'draft') return false
    if (departmentId !== 'all' && request.person.departmentId !== departmentId) return false
    return true
  })

  return (
    <div data-testid="hr-area">
      <p className="kicker">{t('hr.kicker')}</p>
      <h1 className="display">
        <span className="display-text">{t('hr.title')}</span>
      </h1>
      <p className="lead">
        {t('hr.myArea')} <Link to="/">{t('nav.employee')}</Link>
      </p>
      <div className="actions">
        <Link className="btn btn-yellow" to="/hr/reports">
          {t('hr.openReports')}
        </Link>
      </div>
      {loading ? <p className="muted">{t('common.loading')}</p> : null}
      {error ? <p className="banner banner-warn">{translateError(t, error)}</p> : null}

      <section>
        <div className="item-top">
          <h2 className="on-dark-title">{t('hr.queue')}</h2>
          <div className="seg" role="group">
            {filters.map((item) => (
              <button key={item} type="button" aria-pressed={filter === item} onClick={() => setFilter(item)}>
                {item === 'all' ? t('hr.filterAll') : t(`request.type.${item}`)}
              </button>
            ))}
          </div>
        </div>
        {data && pending.length === 0 ? <p className="card empty">{t('hr.emptyQueue')}</p> : null}
        <div className="stack">
          {pending.map((request) => (
            <DecisionCard
              key={request.id}
              request={request}
              approveLabel={t('hr.approve')}
              rejectLabel={t('hr.reject')}
              extra={
                request.type === 'sick' && !request.certificateNumber ? (
                  <p className="banner banner-warn">{t('hr.certificateMissing')}</p>
                ) : null
              }
              onDecide={async (decision, comment) => {
                if (decision === 'approve' && request.type === 'sick' && !request.certificateNumber) {
                  throw new ApiError('certificate_required', 400)
                }
                await api.decide(request.pendingStepId || request.id, {
                  decision,
                  comment: comment || undefined,
                })
                reload()
              }}
            />
          ))}
        </div>
      </section>

      <section className="stack">
        <h2 className="on-dark-title">{t('hr.processedQueue')}</h2>
        {data && data.queue.awaitingProcessed.length === 0 ? <p className="card empty">{t('hr.emptyProcessed')}</p> : null}
        {data?.queue.awaitingProcessed.map((request) => (
          <article key={request.id} className="card item-top">
            <div>
              <strong>{request.person.displayName}</strong>
              <div className="muted">
                {request.person.department} · {formatRange(request.start, request.end, language)}
              </div>
            </div>
            <button className="btn btn-yellow" type="button" onClick={() => void api.markProcessed(request.id).then(() => reload())}>
              {t('hr.markProcessed')}
            </button>
          </article>
        ))}
      </section>

      {data ? (
        <section className="plan">
          <div className="item-top">
            <h2 className="on-dark-title">{t('hr.schedule')}</h2>
            <label className="field inline">
              <span>{t('reports.department')}</span>
              <select value={departmentId} onChange={(event) => setDepartmentId(event.target.value)}>
                <option value="all">{t('hr.departmentAll')}</option>
                {data.departments.map((department) => (
                  <option key={department.id} value={department.id}>
                    {department.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <p className="muted">{t('hr.scheduleHint')}</p>
          <YearCalendar year={year} days={data.days} requests={schedule} />
        </section>
      ) : null}
    </div>
  )
}
