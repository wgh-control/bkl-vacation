import { Link } from 'react-router-dom'
import type { LeaveRequest } from '../api/types'
import { formatDays, formatRange } from '../format'
import { useI18n } from '../i18n/context'
import { StatusPill } from './StatusPill'

export function RequestCard({ request, all }: { request: LeaveRequest; all: LeaveRequest[] }) {
  const { t, language } = useI18n()
  const previous = all.find((item) => item.id === request.replacesRequestId)
  const next = all.find((item) => item.id === request.replacedByRequestId)
  return (
    <Link className="item" to={`/requests/${request.id}`}>
      <div className="item-top">
        <strong>{t(`request.type.${request.type}`)}</strong>
        <StatusPill status={request.status} />
      </div>
      <div>{formatRange(request.start, request.end, language)}</div>
      <div className="muted">{formatDays(language, request.countedDays)}</div>
      {request.nextActor ? <div className="muted">{t(`request.next.${request.nextActor}`)}</div> : null}
      {previous ? (
        <div className="muted">
          {t('detail.movedFrom')} {formatRange(previous.start, previous.end, language)}
        </div>
      ) : null}
      {next ? (
        <div className="muted">
          {t('detail.movedTo')} {formatRange(next.start, next.end, language)}
        </div>
      ) : null}
    </Link>
  )
}
