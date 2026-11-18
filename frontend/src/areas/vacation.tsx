import { EmployeeHome } from '../employee/EmployeeHome'
import { RequestDetail } from '../employee/RequestDetail'
import { RequestWizard } from '../employee/RequestWizard'
import { HrArea } from '../pages/HrArea'
import { HrReports } from '../pages/HrReports'
import { ManagerArea } from '../pages/ManagerArea'
import type { PortalArea } from '../shell/types'

export const vacationArea: PortalArea = {
  id: 'vacation',
  nav: [
    { to: '/', end: true, testId: 'nav-employee', label: 'nav.employee' },
    {
      to: '/team',
      testId: 'nav-manager',
      label: 'nav.manager',
      visible: (me) => me.roles.includes('manager'),
    },
    { to: '/hr', testId: 'nav-hr', label: 'nav.hr', visible: (me) => me.roles.includes('hr') },
  ],
  routes: [
    { index: true, element: <EmployeeHome /> },
    { path: 'requests/new', element: <RequestWizard /> },
    { path: 'requests/:id', element: <RequestDetail /> },
    { path: 'team', role: 'manager', element: <ManagerArea /> },
    { path: 'hr', role: 'hr', element: <HrArea /> },
    { path: 'hr/reports', role: 'hr', element: <HrReports /> },
  ],
}
