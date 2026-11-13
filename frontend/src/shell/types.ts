import type { ReactNode } from 'react'
import type { Me, Role } from '../api/types'
import type { MessageKey } from '../i18n/ru'

export type AreaNavItem = {
  to: string
  end?: boolean
  testId: string
  label: MessageKey
  visible?: (me: Me) => boolean
}

type AreaRouteBase = {
  element: ReactNode
  role?: Role
}

export type AreaRoute = AreaRouteBase & ({ index: true } | { path: string })

/** One product inside the shared shell. It contributes routes and nav entries. */
export type PortalArea = {
  id: string
  nav: AreaNavItem[]
  routes: AreaRoute[]
}
