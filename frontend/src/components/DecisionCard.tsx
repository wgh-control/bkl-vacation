import { useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../api/errors'
import type { LeaveRequest } from '../api/types'
import { formatDays, formatRange } from '../format'
import { translateError, useI18n } from '../i18n/context'
import { StatusPill } from './StatusPill'

export function DecisionCard({
  request,
  approveLabel,
  rejectLabel,
  onDecide,
  extra,
}: {
  request: LeaveRequest
  approveLabel: string
  rejectLabel: string
  onDecide: (decision: 'approve' | 'reject', comment: string) => Promise<void>
  extra?: ReactNode
}) {
  const { t, language } = useI18n()
  const [comment, setComment] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function act(decision: 'approve' | 'reject') {
    setPending(true)
    setError(null)
    try {
      await onDecide(decision, comment)
    } catch (reason) {
      setError(reason instanceof ApiError ? reason.code : 'unknown')
    } finally {
      setPending(false)
    }
  }

  return (
    <article className="card decision">
      <div className="item-top">
        <div>
          <strong>{request.person.displayName}</strong>
          <div className="muted">
            {request.person.department} · {t(`request.type.${request.type}`)}
          </div>
        </div>
        <StatusPill status={request.status} />
      </div>
      <p>
        <Link to={`/requests/${request.id}`}>{formatRange(request.start, request.end, language)}</Link>
        {' · '}
        {formatDays(language, request.countedDays)}
      </p>
      {request.managerDecision ? (
        <p className="muted">
          {t('hr.managerDecision')}: {request.managerDecision.decision === 'approve' ? approveLabel : rejectLabel}
          {request.managerDecision.comment ? ` — ${request.managerDecision.comment}` : ''}
        </p>
      ) : request.status === 'pending_hr' ? (
        <p className="muted">{t('hr.noManagerDecision')}</p>
      ) : null}
      {request.type === 'sick' ? (
        <p className="muted">
          {t('detail.certificate')}: {request.certificateNumber || '—'}
        </p>
      ) : null}
      {request.reasonCode ? <p className="muted">{t(`reason.${request.reasonCode}`)}</p> : null}
      {request.payByWarning && request.type === 'vacation' ? <p className="banner banner-warn">{t('hr.payByWarning')}</p> : null}
      {request.routedToHrBecauseManagerVacant ? <p className="banner banner-info">{t('detail.managerVacant')}</p> : null}
      {extra}
      <label className="field">
        <span>
          {t('request.wizard.comment')} <span className="muted">({t('common.optional')})</span>
        </span>
        <textarea value={comment} onChange={(event) => setComment(event.target.value)} rows={2} />
      </label>
      {error ? <p className="banner banner-warn">{translateError(t, error)}</p> : null}
      <div className="actions">
        <button className="btn btn-primary" type="button" disabled={pending} onClick={() => void act('approve')}>
          {approveLabel}
        </button>
        <button className="btn btn-ghost" type="button" disabled={pending} onClick={() => void act('reject')}>
          {rejectLabel}
        </button>
      </div>
    </article>
  )
}
