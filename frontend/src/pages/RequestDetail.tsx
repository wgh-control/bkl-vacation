import { useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ApiError } from '../api/errors'
import type { LeaveRequest } from '../api/types'
import { todayISO } from '../domain/dates'
import {
  canCancel,
  canReschedule,
  confirmationState,
  noticeDue,
  timelineIndex,
  timelineSteps,
} from '../domain/leave'
import { formatDay, formatDays, formatRange } from '../format'
import { translateError, useI18n } from '../i18n/context'
import { useApi, useMe } from '../session'
import { useLoad } from '../useLoad'
import { StatusPill } from '../components/StatusPill'

export function RequestDetail() {
  const { id = '' } = useParams()
  const api = useApi()
  const me = useMe()
  const navigate = useNavigate()
  const { t, language } = useI18n()
  const today = todayISO()
  const load = useCallback(async () => api.getRequest(id), [api, id])
  const { data: request, error, loading, reload } = useLoad(load, [load])
  const [comment, setComment] = useState('')
  const [start, setStart] = useState('')
  const [end, setEnd] = useState('')
  const [certificate, setCertificate] = useState('')
  const [moving, setMoving] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [decisionComment, setDecisionComment] = useState('')

  async function run(action: () => Promise<unknown>) {
    setActionError(null)
    try {
      await action()
      reload()
    } catch (reason) {
      setActionError(reason instanceof ApiError ? reason.code : 'unknown')
    }
  }

  if (loading) return <p className="muted">{t('common.loading')}</p>
  if (error || !request) return <p className="banner banner-warn">{translateError(t, error ?? 'not_found')}</p>

  const own = request.person.id === me.id
  const confirmation = confirmationState(request, today)
  const steps = timelineSteps(request)
  const current = timelineIndex(request)
  const linkedId = request.replacesRequestId || request.replacedByRequestId

  return (
    <div>
      <Link className="back" to="/">
        {t('common.back')}
      </Link>
      <p className="kicker">{t('detail.kicker')}</p>
      <h1 className="display">
        <span className="display-text">{t(`request.type.${request.type}`)}</span>
      </h1>
      <p className="profile">
        <span>{request.person.displayName}</span>
        <span>{formatRange(request.start, request.end, language)}</span>
        <StatusPill status={request.status} />
      </p>
      <div className="card stack">
        <p>
          {t('detail.counted')}: <strong>{formatDays(language, request.countedDays)}</strong>
        </p>
        {request.payByDate ? (
          <p>
            {t('detail.payBy')}: <strong>{formatDay(request.payByDate, language)}</strong>
          </p>
        ) : null}
        {request.payByWarning ? <p className="banner banner-warn">{t('hr.payByWarning')}</p> : null}
        {request.reasonCode ? (
          <p>
            {t('detail.reason')}: {t(`reason.${request.reasonCode}`)}
          </p>
        ) : null}
        {request.certificateNumber ? (
          <p>
            {t('detail.certificate')}: {request.certificateNumber}
          </p>
        ) : null}
        {request.comment ? <p>{request.comment}</p> : null}
        {request.routedToHrBecauseManagerVacant ? <p className="banner banner-info">{t('detail.managerVacant')}</p> : null}
        {linkedId ? <LinkedDates id={linkedId} request={request} /> : null}
        <h2>{t('detail.timeline')}</h2>
        {request.status === 'rejected' || request.status === 'cancelled' ? <StatusPill status={request.status} /> : null}
        <ol className="timeline">
          {steps.map((step, index) => (
            <li key={step} className={index < current ? 'done' : index === current ? 'current' : ''}>
              {t(`timeline.${step}`)}
            </li>
          ))}
        </ol>
        {request.managerDecision?.comment ? <p className="muted">{request.managerDecision.comment}</p> : null}
        {request.hrDecision?.comment ? <p className="muted">{request.hrDecision.comment}</p> : null}
      </div>

      {actionError ? <p className="banner banner-warn">{translateError(t, actionError)}</p> : null}

      {own && (confirmation === 'ready' || confirmation === 'passed') ? (
        <section className="banner banner-info">
          <p>{confirmation === 'ready' ? t('notice.confirmReady') : t('notice.confirmPassed')}</p>
          <button className="btn btn-primary" type="button" onClick={() => void run(() => api.confirmRequest(request.id))}>
            {t('detail.confirm')}
          </button>
        </section>
      ) : null}
      {own && confirmation === 'upcoming' ? <p className="banner banner-info">{t('notice.confirmUpcoming')}</p> : null}
      {own && noticeDue(request, today) ? (
        <section className="banner banner-info">
          <p>{t('notice.acknowledge')}</p>
          <button className="btn btn-yellow" type="button" onClick={() => void run(() => api.acknowledgeRequest(request.id))}>
            {t('detail.acknowledge')}
          </button>
        </section>
      ) : null}
      {own && request.acknowledgedAt && request.type === 'vacation' ? <p className="muted">{t('detail.acknowledgeDone')}</p> : null}

      {own && request.type === 'sick' && request.status !== 'processed' && request.status !== 'rejected' && request.status !== 'cancelled' ? (
        <form
          className="card stack"
          onSubmit={(event) => {
            event.preventDefault()
            void run(() => api.setCertificate(request.id, certificate))
          }}
        >
          <label className="field">
            <span>{t('detail.certificate')}</span>
            <input value={certificate} onChange={(event) => setCertificate(event.target.value)} required />
          </label>
          <button className="btn btn-primary" type="submit">
            {t('detail.addCertificate')}
          </button>
        </form>
      ) : null}

      {own && request.overlap ? (
        <section className="card stack">
          <h2>{t('detail.overlapTitle')}</h2>
          <p>{formatDays(language, request.overlap.days)}</p>
          {request.overlap.choice ? (
            <>
              <p>{t('detail.overlapSaved')}</p>
              {request.overlap.choice === 'postpone' ? (
                <Link className="btn btn-yellow" to="/requests/new?type=vacation">
                  {t('detail.overlapLater')}
                </Link>
              ) : null}
            </>
          ) : (
            <div className="actions">
              <button
                className="btn btn-primary"
                type="button"
                onClick={() => void run(() => api.chooseOverlap(request.id, 'extend'))}
              >
                {t('detail.overlapExtend')}
              </button>
              <button
                className="btn btn-ghost"
                type="button"
                onClick={() => void run(() => api.chooseOverlap(request.id, 'postpone'))}
              >
                {t('detail.overlapPostpone')}
              </button>
            </div>
          )}
        </section>
      ) : null}

      {request.pendingStepId &&
      ((request.status === 'pending_manager' && me.roles.includes('manager') && !own) ||
        (request.status === 'pending_hr' && me.roles.includes('hr'))) ? (
        <form
          className="card stack"
          onSubmit={(event) => {
            event.preventDefault()
          }}
        >
          <label className="field">
            <span>{t('request.wizard.comment')}</span>
            <textarea value={decisionComment} onChange={(event) => setDecisionComment(event.target.value)} rows={2} />
          </label>
          <div className="actions">
            <button
              className="btn btn-primary"
              type="button"
              onClick={() =>
                void run(() => api.decide(request.pendingStepId!, { decision: 'approve', comment: decisionComment }))
              }
            >
              {t('hr.approve')}
            </button>
            <button
              className="btn btn-ghost"
              type="button"
              onClick={() =>
                void run(() => api.decide(request.pendingStepId!, { decision: 'reject', comment: decisionComment }))
              }
            >
              {t('hr.reject')}
            </button>
          </div>
        </form>
      ) : null}

      {own && canReschedule(request) ? (
        <section className="card stack">
          <h2>{t('detail.reschedule')}</h2>
          <p className="muted">{t('detail.rescheduleNote')}</p>
          {moving ? (
            <form
              className="stack"
              onSubmit={(event) => {
                event.preventDefault()
                void run(async () => {
                  const successor = await api.rescheduleRequest(request.id, { start, end, comment: comment || undefined })
                  navigate(`/requests/${successor.id}`)
                })
              }}
            >
              <div className="fields-2">
                <label className="field">
                  <span>{t('common.from')}</span>
                  <input type="date" required value={start} onChange={(event) => setStart(event.target.value)} />
                </label>
                <label className="field">
                  <span>{t('common.to')}</span>
                  <input type="date" required value={end} min={start} onChange={(event) => setEnd(event.target.value)} />
                </label>
              </div>
              <label className="field">
                <span>{t('request.wizard.comment')}</span>
                <textarea rows={2} value={comment} onChange={(event) => setComment(event.target.value)} />
              </label>
              <button className="btn btn-yellow" type="submit">
                {t('detail.rescheduleSubmit')}
              </button>
            </form>
          ) : (
            <button className="btn btn-yellow" type="button" onClick={() => setMoving(true)}>
              {t('detail.reschedule')}
            </button>
          )}
        </section>
      ) : null}

      {own && canCancel(request) ? (
        <section className="stack">
          {cancelling ? (
            <div className="actions">
              <span>{t('detail.cancelConfirm')}</span>
              <button className="btn btn-primary" type="button" onClick={() => void run(() => api.cancelRequest(request.id))}>
                {t('detail.cancelYes')}
              </button>
              <button className="btn btn-ghost-light" type="button" onClick={() => setCancelling(false)}>
                {t('common.back')}
              </button>
            </div>
          ) : (
            <button className="btn btn-ghost-light" type="button" onClick={() => setCancelling(true)}>
              {t('detail.cancel')}
            </button>
          )}
        </section>
      ) : null}
    </div>
  )
}

function LinkedDates({ id, request }: { id: string; request: LeaveRequest }) {
  const api = useApi()
  const { t, language } = useI18n()
  const load = useCallback(async () => api.getRequest(id), [api, id])
  const { data } = useLoad(load, [load])
  if (!data) return null
  const label = request.replacesRequestId === id ? t('detail.movedFrom') : t('detail.movedTo')
  return (
    <p>
      {label} <Link to={`/requests/${data.id}`}>{formatRange(data.start, data.end, language)}</Link>
    </p>
  )
}
