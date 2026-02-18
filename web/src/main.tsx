import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App'

const basePath = ((window as any).__BASE_PATH__ || '').replace(/\/+$/, '')

if ('serviceWorker' in navigator && window.isSecureContext) {
  window.addEventListener('load', () => {
    const swPath = `${basePath}/sw.js`
    const scope = basePath ? `${basePath}/` : '/'
    navigator.serviceWorker.register(swPath, { scope }).catch(() => {
      // Non-fatal: dashboard works without service worker.
    })
  })
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
