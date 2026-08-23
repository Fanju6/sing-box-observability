import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { server } from '@/mocks/node'
import { createHandlers } from '@/mocks/handlers'
import { describe, expect, it } from 'vitest'
import { AppProviders } from '@/app/providers'
import { BrowserRouter } from 'react-router-dom'
import { LoginPage } from './login-page'
import { api } from '@/api/client'

describe('console session flow', () => {
  it('redirects to login when enabled and enters the app after success', async () => {
    server.use(...createHandlers(true))
    expect(await api.getSession()).toEqual({ authEnabled: true, authenticated: false })
    window.history.replaceState({}, '', '/login')
    const user = userEvent.setup()
    render(<AppProviders><BrowserRouter><LoginPage /></BrowserRouter></AppProviders>)

    const token = await screen.findByPlaceholderText(/console token|控制台 token/i)
    await user.type(token, 'token')
    await user.click(screen.getByRole('button', { name: /sign in|登录/i }))

    await waitFor(() => expect(window.location.pathname).toBe('/overview'))
  }, 10_000)
})
