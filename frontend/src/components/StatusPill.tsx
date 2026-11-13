import type { RequestStatus } from '../api/types'
import { useI18n } from '../i18n/context'

const tone: Record<RequestStatus, string> = {
  draft: 'pill-move',
  pending_manager: 'pill-pending',
  pending_hr: 'pill-pending',
  scheduled: 'pill-ok',
  awaiting_confirmation: 'pill-pending',
  confirmed: 'pill-ok',
  processed: 'pill-done',
  rejected: 'pill-bad',
  cancelled: 'pill-bad',
}

export function StatusPill({ status }: { status: RequestStatus }) {
  const { t } = useI18n()
  return <span className={`pill ${tone[status]}`}>{t(`request.status.${status}`)}</span>
}
