import { useCallback, useMemo, useState } from 'react'
import { ApiError } from '../api/errors'
import type { ManagerChoice, PersonRef } from '../api/types'
import { translateError, useI18n } from '../i18n/context'
import { useApi } from '../session'
import { useLoad } from '../useLoad'

export function HrManagers() {
  const api = useApi()
  const { t } = useI18n()
  const load = useCallback(() => api.getManagerDirectory(), [api])
  const { data, error, loading, reload } = useLoad(load, [load])
  const [choice, setChoice] = useState<Record<string, string>>({})
  const [actionError, setActionError] = useState<string | null>(null)
  const options = useMemo(() => {
    const people = new Map<string, PersonRef>()
    for (const group of data?.groups ?? []) {
      if (group.manager) people.set(group.manager.id, group.manager)
      for (const person of group.people) people.set(person.id, { id: person.id, displayName: person.displayName })
    }
    return [...people.values()].sort((a, b) => a.displayName.localeCompare(b.displayName, 'ru'))
  }, [data])

  async function act(employeeId: string, action: ManagerChoice, managerId?: string) {
    setActionError(null)
    try {
      await api.setManager(employeeId, { action, managerId })
      setChoice((current) => ({ ...current, [employeeId]: '' }))
      reload()
    } catch (reason) {
      setActionError(reason instanceof ApiError ? reason.code : 'unknown')
    }
  }

  return (
    <section data-testid="hr-managers" className="stack">
      <div>
        <h2 className="on-dark-title">{t('hr.managers')}</h2>
        <p className="muted">{t('hr.managersHint')}</p>
      </div>
      {loading ? <p className="muted">{t('common.loading')}</p> : null}
      {error ? <p className="banner banner-warn">{translateError(t, error)}</p> : null}
      {actionError ? <p className="banner banner-warn">{translateError(t, actionError)}</p> : null}
      {data?.groups.map((group) => (
        <section key={group.manager?.id ?? 'none'} className="card stack" data-testid="manager-group">
          <h3>{group.manager?.displayName ?? t('hr.unassigned')}</h3>
          {group.people.map((person) => {
            const directoryName = person.adManager?.displayName || person.adManagerRaw
            const changeName = person.adChange?.displayName || person.adChangeRaw
            return (
              <article key={person.id} className="managed" data-testid={`managed-${person.id}`}>
                <div className="item-top">
                  <strong>{person.displayName}</strong>
                  {person.manual ? <span className="muted">{t('hr.manual')}</span> : null}
                </div>
                {person.position || person.department ? (
                  <p className="muted">
                    {[person.position, person.department].filter(Boolean).join(' · ')}
                  </p>
                ) : null}
                {person.manual && directoryName ? (
                  <p className="muted">
                    {t('hr.fromDirectory')}: {directoryName}
                  </p>
                ) : null}
                {changeName ? (
                  <div className="banner banner-info">
                    <p>
                      {t('hr.adChange')} <strong>{changeName}</strong>
                    </p>
                    <div className="actions">
                      <button className="btn btn-yellow btn-small" type="button" onClick={() => void act(person.id, 'take_ad')}>
                        {t('hr.takeAd')}
                      </button>
                      <button className="btn btn-ghost btn-small" type="button" onClick={() => void act(person.id, 'keep')}>
                        {t('hr.keepManager')}
                      </button>
                    </div>
                  </div>
                ) : null}
                <div className="actions">
                  <label className="field inline">
                    <span>{t('hr.chooseManager')}</span>
                    <select
                      aria-label={`${t('hr.chooseManager')} ${person.displayName}`}
                      value={choice[person.id] ?? ''}
                      onChange={(event) => setChoice((current) => ({ ...current, [person.id]: event.target.value }))}
                    >
                      <option value="">{t('hr.noManagerOption')}</option>
                      {options
                        .filter((option) => option.id !== person.id)
                        .map((option) => (
                          <option key={option.id} value={option.id}>
                            {option.displayName}
                          </option>
                        ))}
                    </select>
                  </label>
                  <button
                    className="btn btn-primary btn-small"
                    type="button"
                    disabled={!choice[person.id]}
                    onClick={() => void act(person.id, 'set', choice[person.id])}
                  >
                    {t('hr.setManager')}
                  </button>
                  {person.manual ? (
                    <button className="btn btn-ghost btn-small" type="button" onClick={() => void act(person.id, 'clear')}>
                      {t('hr.clearManager')}
                    </button>
                  ) : null}
                </div>
              </article>
            )
          })}
        </section>
      ))}
    </section>
  )
}
