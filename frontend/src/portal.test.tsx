import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeAll, describe, expect, it } from 'vitest'
import { App } from './App'
import { createFixtureApi, type FixtureApi } from './api/fixture'
import { I18nProvider } from './i18n/context'
import { SessionProvider } from './session'

function renderPortal(path = '/') {
  const api: FixtureApi = createFixtureApi({ today: '2026-11-13' })
  const view = render(
    <I18nProvider initial="ru">
      <SessionProvider value={{ api, preview: { setRoles: (roles) => api.setRoles(roles) }, logout: () => undefined }}>
        <MemoryRouter initialEntries={[path]}>
          <App />
        </MemoryRouter>
      </SessionProvider>
    </I18nProvider>,
  )
  return { api, ...view }
}

describe('portal areas', () => {
  afterEach(() => cleanup())

  beforeAll(() => {
    if (typeof URL.createObjectURL !== 'function') URL.createObjectURL = () => 'blob:test'
    URL.revokeObjectURL = () => undefined
  })

  it('shows planned, approved, and moved days and opens next year’s vacation', async () => {
    const user = userEvent.setup()
    renderPortal('/')
    expect(await screen.findByTestId('bucket-planned')).toBeTruthy()
    expect(screen.getByTestId('bucket-approved')).toBeTruthy()
    expect(screen.getByTestId('bucket-moved')).toBeTruthy()
    expect(screen.getByText('Запланированные')).toBeTruthy()
    expect(screen.getByText('Согласованные')).toBeTruthy()
    expect(screen.getByText('Перенесённые')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: '2025 Прошлый' }))
    expect(await screen.findByRole('heading', { name: 'План на год 2025' })).toBeTruthy()
    expect(screen.getByText(/Прошлый год только для просмотра/)).toBeTruthy()
    expect(screen.queryByRole('link', { name: 'Новая заявка' })).toBeNull()

    await user.click(screen.getByTestId('plan-next-year'))
    expect(await screen.findByRole('heading', { name: 'Отпуск на следующий год' })).toBeTruthy()
    const submit = await screen.findByRole('button', { name: 'Отправить заявку' })
    await waitFor(() => expect((submit as HTMLButtonElement).disabled).toBe(false))
    await user.click(submit)
    expect(await screen.findByText('У руководителя')).toBeTruthy()
  })

  it('switches the employee area to English', async () => {
    const user = userEvent.setup()
    renderPortal('/')
    await screen.findByTestId('employee-area')
    await user.click(screen.getByRole('button', { name: 'EN' }))
    expect(await screen.findByRole('heading', { name: 'My area' })).toBeTruthy()
    expect(screen.getByText('Planned')).toBeTruthy()
    expect(screen.getByText('Approved')).toBeTruthy()
    expect(screen.getByText('Moved')).toBeTruthy()
  })

  it('keeps the employee area for an HR user and downloads both report formats', async () => {
    const user = userEvent.setup()
    renderPortal('/hr')
    expect(await screen.findByTestId('hr-area')).toBeTruthy()
    expect(screen.getByTestId('nav-employee')).toBeTruthy()
    expect(screen.getAllByText('Отпуск').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Отгул').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Больничный').length).toBeGreaterThan(0)

    await user.click(screen.getByRole('link', { name: 'Скачать PDF и XLS' }))
    expect(await screen.findByTestId('hr-reports')).toBeTruthy()
    await user.click(screen.getByTestId('download-pdf'))
    expect(await screen.findByText(/bkl-leave-all\.pdf/)).toBeTruthy()
    await user.click(screen.getByTestId('download-xls'))
    expect(await screen.findByText(/bkl-leave-all\.xls/)).toBeTruthy()

    await user.click(screen.getByTestId('report-scope-person'))
    await user.click(screen.getByRole('radio', { name: /Иванов Пётр/ }))
    await user.click(screen.getByTestId('download-pdf'))
    expect(await screen.findByText(/bkl-leave-person\.pdf/)).toBeTruthy()

    await user.click(screen.getByTestId('nav-employee'))
    expect(await screen.findByTestId('employee-area')).toBeTruthy()
  })

  it('hides the HR area when the signed-in person is only an employee', async () => {
    const user = userEvent.setup()
    renderPortal('/')
    await screen.findByTestId('nav-hr')
    await user.click(screen.getByTestId('preview-role-hr'))
    await waitFor(() => expect(screen.queryByTestId('nav-hr')).toBeNull())
    expect(screen.getByTestId('nav-employee')).toBeTruthy()
  })
})
