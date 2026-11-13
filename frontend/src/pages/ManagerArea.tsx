import { useCallback, useState } from 'react'
import { DecisionCard } from '../components/DecisionCard'
import { YearCalendar } from '../components/YearCalendar'
import { calendarYear, todayISO, yearBounds } from '../domain/dates'
import { translateError, useI18n } from '../i18n/context'
import { useApi } from '../session'
import { useLoad } from '../useLoad'

export function ManagerArea() {
  const api = useApi()
  const { t } = useI18n()
  const today = todayISO()
  const year = calendarYear(today)
  const { from, to } = yearBounds(year)
  const [error, setError] = useState<string | null>(null)
  const load = useCallback(async () => {
    const [approvals, requests, days] = await Promise.all([
      api.getApprovals(),
      api.getRequests({ scope: 'reports', from, to }),
      api.getCalendar(from, to),
    ])
    return { approvals: approvals.filter((item) => item.role === 'manager'), requests, days }
  }, [api, from, to])
  const { data, error: loadError, loading, reload } = useLoad(load, [load])

  return (
    <div data-testid="manager-area">
      <p className="kicker">{t('manager.kicker')}</p>
      <h1 className="display">
        <span className="display-text">{t('manager.title')}</span>
      </h1>
      {loading ? <p className="muted">{t('common.loading')}</p> : null}
      {loadError || error ? <p className="banner banner-warn">{translateError(t, loadError || error || 'unknown')}</p> : null}
      {data && data.approvals.length === 0 ? <p className="card empty">{t('manager.empty')}</p> : null}
      <div className="stack">
        {data?.approvals.map((item) => (
          <DecisionCard
            key={item.id}
            request={item.request}
            approveLabel={t('manager.approve')}
            rejectLabel={t('manager.reject')}
            onDecide={async (decision, comment) => {
              try {
                await api.decide(item.id, { decision, comment: comment || undefined })
                setError(null)
                reload()
              } catch (reason) {
                setError(reason instanceof Error && 'code' in reason ? String((reason as { code: string }).code) : 'unknown')
                throw reason
              }
            }}
          />
        ))}
      </div>
      {data ? (
        <section className="plan">
          <h2 className="on-dark-title">{t('manager.calendar')}</h2>
          <YearCalendar
            year={year}
            days={data.days}
            requests={data.requests.filter((request) => request.status !== 'rejected' && request.status !== 'cancelled')}
          />
        </section>
      ) : null}
    </div>
  )
}
