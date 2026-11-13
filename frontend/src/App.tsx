import { Navigate, Route, Routes } from 'react-router-dom'
import type { Role } from './api/types'
import { Shell } from './components/Shell'
import { EmployeeHome } from './pages/EmployeeHome'
import { HrArea } from './pages/HrArea'
import { HrReports } from './pages/HrReports'
import { ManagerArea } from './pages/ManagerArea'
import { RequestDetail } from './pages/RequestDetail'
import { RequestWizard } from './pages/RequestWizard'
import { useMe } from './session'
import type { ReactNode } from 'react'

function RequireRole({ role, children }: { role: Role; children: ReactNode }) {
  const me = useMe()
  if (!me.roles.includes(role)) return <Navigate to="/" replace />
  return children
}

export function App() {
  return (
    <Routes>
      <Route element={<Shell />}>
        <Route index element={<EmployeeHome />} />
        <Route path="requests/new" element={<RequestWizard />} />
        <Route path="requests/:id" element={<RequestDetail />} />
        <Route
          path="team"
          element={
            <RequireRole role="manager">
              <ManagerArea />
            </RequireRole>
          }
        />
        <Route
          path="hr"
          element={
            <RequireRole role="hr">
              <HrArea />
            </RequireRole>
          }
        />
        <Route
          path="hr/reports"
          element={
            <RequireRole role="hr">
              <HrReports />
            </RequireRole>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
