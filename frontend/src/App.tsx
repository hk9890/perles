import { useState, useEffect, useRef } from 'react'
import type { Session } from './types'
import SessionLoader from './components/SessionLoader'
import SessionViewer from './components/SessionViewer'
import './App.css'

function App() {
  const [session, setSession] = useState<Session | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null)
  // Force re-render counter for real-time timestamp updates
  const [, forceUpdate] = useState(0)
  const [initialLoading, setInitialLoading] = useState(() => {
    // Check if we have a path in URL - if so, we'll be loading
    const params = new URLSearchParams(window.location.search)
    return !!params.get('path')
  })

  // Refs for polling interval management
  const intervalRef = useRef<number | null>(null)
  const isPollingRef = useRef<boolean>(false)

  // Load session from URL param on mount
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const pathFromUrl = params.get('path')
    if (pathFromUrl) {
      handleLoad(pathFromUrl).finally(() => setInitialLoading(false))
    }
  }, [])

  // Polling interval for session data refresh
  useEffect(() => {
    if (!session) {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
      return
    }

    const poll = async () => {
      if (isPollingRef.current) return
      isPollingRef.current = true
      try {
        await handleLoad(session.path)
      } finally {
        isPollingRef.current = false
      }
    }

    intervalRef.current = window.setInterval(poll, 5000)

    // Page Visibility API: pause polling when tab is hidden
    const handleVisibilityChange = () => {
      if (document.hidden) {
        if (intervalRef.current) {
          clearInterval(intervalRef.current)
          intervalRef.current = null
        }
      } else if (session?.path) {
        // Restart polling and do immediate refresh
        poll()
        intervalRef.current = window.setInterval(poll, 5000)
      }
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [session?.path])

  // Real-time timestamp updates every second
  useEffect(() => {
    if (!lastRefreshed) return
    const timer = setInterval(() => forceUpdate(n => n + 1), 1000)
    return () => clearInterval(timer)
  }, [lastRefreshed])

  const handleLoad = async (path: string) => {
    setError(null)
    try {
      const res = await fetch('/api/load-session', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path })
      })
      
      const data = await res.json()
      
      if (!res.ok) {
        throw new Error(data.error || 'Failed to load session')
      }
      
      setSession(data)
      setLastRefreshed(new Date())

      // Update URL with session path
      const url = new URL(window.location.href)
      url.searchParams.set('path', path)
      window.history.replaceState({}, '', url.toString())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  const handleClear = () => {
    setSession(null)
    setError(null)
    setLastRefreshed(null)

    // Clear URL params
    const url = new URL(window.location.href)
    url.searchParams.delete('path')
    url.searchParams.delete('tab')
    window.history.replaceState({}, '', url.toString())
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>🔮 Perles Session Viewer</h1>
        {session && (
          <div className="header-right">
            <code className="session-id-display">{session.metadata?.session_id}</code>

            <button className="clear-btn" onClick={handleClear}>
              Load Different Session
            </button>
          </div>
        )}
      </header>
      
      <main className="app-main">
        {initialLoading ? (
          <div className="initial-loading">Loading session...</div>
        ) : !session ? (
          <SessionLoader onLoad={handleLoad} error={error} />
        ) : (
          <SessionViewer session={session} />
        )}
      </main>
    </div>
  )
}

export default App
