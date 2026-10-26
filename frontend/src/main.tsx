import { createRoot } from 'react-dom/client'
import { App } from './app/App'
import { I18nProvider } from './lib/i18n'
import { initTheme } from './app/theme'
import '@xterm/xterm/css/xterm.css'
import './app/styles.css'

initTheme()
createRoot(document.getElementById('root')!).render(<I18nProvider><App /></I18nProvider>)
