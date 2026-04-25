import React from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App'
import { ThemeProvider } from './theme/theme-provider'

class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  { error: Error | null }
> {
  constructor(props: { children: React.ReactNode }) {
    super(props);
    this.state = { error: null };
  }

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: '2rem', color: '#f87171', fontFamily: 'monospace', background: '#09090b', minHeight: '100vh' }}>
          <h2 style={{ marginBottom: '0.5rem' }}>PairAdmin encountered an error</h2>
          <pre style={{ fontSize: '0.75rem', whiteSpace: 'pre-wrap' }}>{this.state.error.message}</pre>
        </div>
      );
    }
    return this.props.children;
  }
}

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
  <React.StrictMode>
    <ErrorBoundary>
      <ThemeProvider defaultTheme="dark" storageKey="pairadmin-theme">
        <App />
      </ThemeProvider>
    </ErrorBoundary>
  </React.StrictMode>
)
