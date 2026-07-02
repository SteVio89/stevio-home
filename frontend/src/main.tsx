import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './i18n'
import '@fortawesome/fontawesome-free/css/all.css'
import './index.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
