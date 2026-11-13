import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import type { Role } from '../api/types'
import { useMe } from '../session'

export function RequireRole({ role, children }: { role: Role; children: ReactNode }) {
  const me = useMe()
  if (!me.roles.includes(role)) return <Navigate to="/" replace />
  return children
}
