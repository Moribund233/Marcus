import React from 'react'
import {createRoot} from 'react-dom/client'
import './globals.css'
import App from './App'
import { I18nProvider } from '@/hooks/useI18n'

const container = document.getElementById('root')
const root = createRoot(container!)

root.render(
  <React.StrictMode>
    <I18nProvider>
      <App/>
    </I18nProvider>
  </React.StrictMode>
)
