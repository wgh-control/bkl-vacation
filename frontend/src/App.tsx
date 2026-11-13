import { Navigate, Route, Routes } from 'react-router-dom'
import { Shell } from './components/Shell'
import { areaRouteElements, AreasProvider, portalAreas } from './shell/registry'
import type { PortalArea } from './shell/types'

export function App({ areas = portalAreas }: { areas?: readonly PortalArea[] }) {
  return (
    <AreasProvider areas={areas}>
      <Routes>
        <Route element={<Shell />}>
          {areaRouteElements(areas)}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </AreasProvider>
  )
}
