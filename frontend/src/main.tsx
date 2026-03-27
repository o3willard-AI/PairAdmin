import React from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App'
import { ThemeProvider } from './theme/theme-provider'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
  <React.StrictMode>
    <ThemeProvider defaultTheme="dark" storageKey="pairadmin-theme">
      <App />
    </ThemeProvider>
  </React.StrictMode>
)
