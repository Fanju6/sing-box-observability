import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './app/router'
import './styles/fonts.css'
import './styles/globals.css'

async function enableMocking() {
  if (!import.meta.env.DEV || import.meta.env.VITE_MSW !== 'true') return
  const { worker } = await import('./mocks/browser')
  return worker.start({
    onUnhandledRequest: 'bypass',
    quiet: true,
  })
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
})
