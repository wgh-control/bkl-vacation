import { createContext, useContext, type ReactNode } from 'react'
import type { Me, Role, VacationApi } from './api/types'

export type PreviewControls = {
  setRoles: (roles: Role[]) => void
}

export type Session = {
  api: VacationApi
  preview: PreviewControls | null
  logout: () => void
}

const SessionContext = createContext<Session | null>(null)

export function SessionProvider({ value, children }: { value: Session; children: ReactNode }) {
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export function useSession(): Session {
  const value = useContext(SessionContext)
  if (!value) throw new Error('session')
  return value
}

export function useApi(): VacationApi {
  return useSession().api
}

const MeContext = createContext<Me | null>(null)

export function MeProvider({ me, children }: { me: Me; children: ReactNode }) {
  return <MeContext.Provider value={me}>{children}</MeContext.Provider>
}

export function useMe(): Me {
  const me = useContext(MeContext)
  if (!me) throw new Error('me')
  return me
}
