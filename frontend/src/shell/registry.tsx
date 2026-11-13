import { createContext, useContext, type ReactNode } from 'react'
import { Route } from 'react-router-dom'
import { vacationArea } from '../areas/vacation'
import { RequireRole } from './RequireRole'
import type { AreaRoute, PortalArea } from './types'

const AreasContext = createContext<readonly PortalArea[] | null>(null)

export function AreasProvider({ areas, children }: { areas: readonly PortalArea[]; children: ReactNode }) {
  return <AreasContext.Provider value={areas}>{children}</AreasContext.Provider>
}

export function useAreas(): readonly PortalArea[] {
  const areas = useContext(AreasContext)
  if (!areas) throw new Error('areas')
  return areas
}

// Append an area here. It adds its routes and nav entries to the shared shell.
export const portalAreas: readonly PortalArea[] = [vacationArea]

function guard(route: AreaRoute): ReactNode {
  if (!route.role) return route.element
  return <RequireRole role={route.role}>{route.element}</RequireRole>
}

export function areaRouteElements(areas: readonly PortalArea[]) {
  return areas.flatMap((area) =>
    area.routes.map((route) => {
      const element = guard(route)
      if ('path' in route) {
        return <Route key={`${area.id}:${route.path}`} path={route.path} element={element} />
      }
      return <Route key={`${area.id}:index`} index element={element} />
    }),
  )
}
