import { useEffect, useState } from 'react'
import './Toast.css'

export interface ToastProps {
  message: string
  type?: 'error' | 'success' | 'info'
  duration?: number  // Auto-dismiss after this many ms (default 5000, 0 = no auto-dismiss)
  onDismiss: () => void
}

export default function Toast({
  message,
  type = 'error',
  duration = 5000,
  onDismiss,
}: ToastProps) {
  const [isVisible, setIsVisible] = useState(true)

  useEffect(() => {
    if (duration > 0) {
      const timer = setTimeout(() => {
        setIsVisible(false)
        // Allow animation to complete before calling onDismiss
        setTimeout(onDismiss, 200)
      }, duration)
      return () => clearTimeout(timer)
    }
  }, [duration, onDismiss])

  const handleDismiss = () => {
    setIsVisible(false)
    setTimeout(onDismiss, 200)
  }

  return (
    <div
      className={`toast toast-${type} ${isVisible ? 'visible' : 'hidden'}`}
      role="alert"
      aria-live="polite"
    >
      <span className="toast-message">{message}</span>
      <button
        className="toast-dismiss"
        onClick={handleDismiss}
        aria-label="Dismiss notification"
      >
        &times;
      </button>
    </div>
  )
}
