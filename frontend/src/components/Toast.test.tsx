import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Toast from './Toast'

describe('Toast', () => {
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  describe('rendering', () => {
    it('renders with message', () => {
      render(<Toast message="Test error message" onDismiss={vi.fn()} />)
      expect(screen.getByText('Test error message')).toBeInTheDocument()
    })

    it('renders with error type by default', () => {
      render(<Toast message="Error" onDismiss={vi.fn()} />)
      expect(screen.getByRole('alert')).toHaveClass('toast-error')
    })

    it('renders with success type', () => {
      render(<Toast message="Success" type="success" onDismiss={vi.fn()} />)
      expect(screen.getByRole('alert')).toHaveClass('toast-success')
    })

    it('renders with info type', () => {
      render(<Toast message="Info" type="info" onDismiss={vi.fn()} />)
      expect(screen.getByRole('alert')).toHaveClass('toast-info')
    })

    it('has correct ARIA attributes', () => {
      render(<Toast message="Test" onDismiss={vi.fn()} />)
      const alert = screen.getByRole('alert')
      expect(alert).toHaveAttribute('aria-live', 'polite')
    })
  })

  describe('auto-dismiss', () => {
    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true })
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    it('auto-dismisses after 5 seconds by default', async () => {
      const onDismiss = vi.fn()
      render(<Toast message="Test" onDismiss={onDismiss} />)

      // Advance timers by 5 seconds (auto-dismiss) + 200ms (animation)
      await vi.advanceTimersByTimeAsync(5200)

      expect(onDismiss).toHaveBeenCalled()
    })

    it('auto-dismisses after custom duration', async () => {
      const onDismiss = vi.fn()
      render(<Toast message="Test" duration={2000} onDismiss={onDismiss} />)

      // Should not be called yet
      await vi.advanceTimersByTimeAsync(1500)
      expect(onDismiss).not.toHaveBeenCalled()

      // Advance past duration + animation
      await vi.advanceTimersByTimeAsync(700)

      expect(onDismiss).toHaveBeenCalled()
    })

    it('does not auto-dismiss when duration is 0', async () => {
      const onDismiss = vi.fn()
      render(<Toast message="Test" duration={0} onDismiss={onDismiss} />)

      await vi.advanceTimersByTimeAsync(10000)

      expect(onDismiss).not.toHaveBeenCalled()
    })
  })

  describe('manual dismiss', () => {
    it('calls onDismiss when dismiss button clicked', async () => {
      const user = userEvent.setup()
      const onDismiss = vi.fn()
      render(<Toast message="Test" duration={0} onDismiss={onDismiss} />)

      const dismissButton = screen.getByRole('button', { name: 'Dismiss notification' })
      await user.click(dismissButton)

      await waitFor(() => {
        expect(onDismiss).toHaveBeenCalled()
      })
    })
  })

  describe('visibility transition', () => {
    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true })
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    it('starts visible', () => {
      render(<Toast message="Test" onDismiss={vi.fn()} />)
      expect(screen.getByRole('alert')).toHaveClass('visible')
    })

    it('becomes hidden before dismiss', async () => {
      render(<Toast message="Test" duration={1000} onDismiss={vi.fn()} />)

      // Advance past duration but not animation
      await vi.advanceTimersByTimeAsync(1000)

      expect(screen.getByRole('alert')).toHaveClass('hidden')
    })
  })
})
