import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ChatInput from './ChatInput'

const defaultAgents = ['coordinator', 'worker-1', 'worker-2', 'worker-3']

const defaultProps = {
  channelSlug: 'general',
  onSend: vi.fn().mockResolvedValue(undefined),
  agentIds: defaultAgents,
}

function renderComponent(props = {}) {
  return render(<ChatInput {...defaultProps} {...props} />)
}

// Helper to get textarea
function getTextarea() {
  return screen.getByRole('textbox')
}

describe('ChatInput', () => {
  beforeEach(() => {
    // Reset window.innerHeight for position calculations in MentionAutocomplete
    Object.defineProperty(window, 'innerHeight', { value: 768, writable: true })
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  describe('basic rendering', () => {
    it('renders textarea and submit button', () => {
      renderComponent()
      expect(getTextarea()).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Send message' })).toBeInTheDocument()
    })

    it('renders with custom placeholder', () => {
      renderComponent({ placeholder: 'Custom placeholder' })
      expect(screen.getByPlaceholderText('Custom placeholder')).toBeInTheDocument()
    })
  })

  describe('submit behavior', () => {
    it('Enter submits message', async () => {
      const onSend = vi.fn().mockResolvedValue(undefined)
      const user = userEvent.setup()
      renderComponent({ onSend })

      const textarea = getTextarea()
      await user.type(textarea, 'Hello world')
      await user.keyboard('{Enter}')

      expect(onSend).toHaveBeenCalledWith('Hello world', [])
    })

    it('Shift+Enter inserts newline (does not submit)', async () => {
      const onSend = vi.fn().mockResolvedValue(undefined)
      const user = userEvent.setup()
      renderComponent({ onSend })

      const textarea = getTextarea()
      await user.type(textarea, 'Line 1')
      await user.keyboard('{Shift>}{Enter}{/Shift}')
      await user.type(textarea, 'Line 2')

      expect(onSend).not.toHaveBeenCalled()
      expect(textarea).toHaveValue('Line 1\nLine 2')
    })

    it('submit button disabled when value is empty', () => {
      renderComponent()
      const button = screen.getByRole('button', { name: 'Send message' })
      expect(button).toBeDisabled()
    })

    it('submit button disabled when value is whitespace only', async () => {
      const user = userEvent.setup()
      renderComponent()

      const textarea = getTextarea()
      await user.type(textarea, '   ')

      const button = screen.getByRole('button', { name: 'Send message' })
      expect(button).toBeDisabled()
    })

    it('content cleared after successful send', async () => {
      const onSend = vi.fn().mockResolvedValue(undefined)
      const user = userEvent.setup()
      renderComponent({ onSend })

      const textarea = getTextarea()
      await user.type(textarea, 'Test message')
      await user.keyboard('{Enter}')

      await waitFor(() => {
        expect(textarea).toHaveValue('')
      })
    })

    it('content preserved after failed send', async () => {
      const onSend = vi.fn().mockRejectedValue(new Error('Network error'))
      const user = userEvent.setup()
      renderComponent({ onSend })

      const textarea = getTextarea()
      await user.type(textarea, 'Test message')
      await user.keyboard('{Enter}')

      await waitFor(() => {
        expect(textarea).toHaveValue('Test message')
      })
    })
  })

  describe('mentions extraction', () => {
    it('extracts mentions from content (when no autocomplete)', async () => {
      // Use agentIds=[] so autocomplete doesn't trigger
      const onSend = vi.fn().mockResolvedValue(undefined)
      renderComponent({ onSend, agentIds: [] })

      const textarea = getTextarea()
      // Use fireEvent instead of userEvent to avoid triggering autocomplete logic character-by-character
      fireEvent.change(textarea, { target: { value: 'Hello @coordinator and @worker-1' } })
      fireEvent.keyDown(textarea, { key: 'Enter' })

      await waitFor(() => {
        expect(onSend).toHaveBeenCalledWith('Hello @coordinator and @worker-1', ['coordinator', 'worker-1'])
      })
    })
  })

  describe('character counter', () => {
    it('character counter appears when >8000 chars', async () => {
      renderComponent()

      const textarea = getTextarea()
      // Use fireEvent instead of userEvent for large text
      const longText = 'a'.repeat(8001)
      fireEvent.change(textarea, { target: { value: longText } })

      expect(screen.getByText('8001/10000')).toBeInTheDocument()
    })

    it('character counter hidden when <=8000 chars', async () => {
      const user = userEvent.setup()
      renderComponent()

      const textarea = getTextarea()
      await user.type(textarea, 'Short text')

      expect(screen.queryByText(/\/10000/)).not.toBeInTheDocument()
    })

    it('shows warning style when >9500 chars', () => {
      renderComponent()

      const textarea = getTextarea()
      const nearLimitText = 'a'.repeat(9501)
      fireEvent.change(textarea, { target: { value: nearLimitText } })

      const counter = screen.getByText('9501/10000')
      expect(counter).toHaveClass('near-limit')
    })

    it('cannot exceed 10000 character limit', () => {
      renderComponent()

      const textarea = getTextarea()
      const overLimitText = 'a'.repeat(10500)
      fireEvent.change(textarea, { target: { value: overLimitText } })

      // Should be truncated to 10000
      expect(textarea).toHaveValue('a'.repeat(10000))
      expect(screen.getByText('10000/10000')).toBeInTheDocument()
    })
  })

  describe('disabled state', () => {
    it('input disabled when disabled prop is true', () => {
      renderComponent({ disabled: true })
      expect(getTextarea()).toBeDisabled()
    })

    it('displays disabled reason text', () => {
      renderComponent({ disabled: true, disabledReason: 'Custom reason' })
      expect(screen.getByText('Custom reason')).toBeInTheDocument()
    })
  })

  describe('@ trigger detection', () => {
    it('@ preceded by space triggers autocomplete', async () => {
      renderComponent()

      const textarea = getTextarea()
      // Use fireEvent with selectionStart for proper cursor tracking
      fireEvent.change(textarea, {
        target: { value: 'hello @', selectionStart: 7 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })
    })

    it('@ at input start triggers autocomplete', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })
    })

    it('@ in middle of word does NOT trigger (e.g., "email@example")', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: 'email@', selectionStart: 6 }
      })

      // MentionAutocomplete should NOT appear
      expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
    })

    it('@ after newline triggers autocomplete', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: 'line1\n@', selectionStart: 7 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })
    })
  })

  describe('query updates as user types after @', () => {
    it('query updates as user types after @', async () => {
      renderComponent()

      const textarea = getTextarea()

      // First trigger autocomplete
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Now type more to filter
      fireEvent.change(textarea, {
        target: { value: '@work', selectionStart: 5 }
      })

      await waitFor(() => {
        // Should show filtered results (workers only)
        expect(screen.getByText('@worker-1')).toBeInTheDocument()
        expect(screen.getByText('@worker-2')).toBeInTheDocument()
        expect(screen.getByText('@worker-3')).toBeInTheDocument()
        // Coordinator should be filtered out
        expect(screen.queryByText('@coordinator')).not.toBeInTheDocument()
      })
    })

    it('empty query shows all agents', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        // Should show all agents
        expect(screen.getByText('@coordinator')).toBeInTheDocument()
        expect(screen.getByText('@worker-1')).toBeInTheDocument()
      })
    })
  })

  describe('autocomplete dismissal', () => {
    it('typing space after @ dismisses autocomplete', async () => {
      renderComponent()

      const textarea = getTextarea()

      // First trigger autocomplete
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Now type space
      fireEvent.change(textarea, {
        target: { value: '@ ', selectionStart: 2 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
      })
    })

    it('backspace past @ dismisses autocomplete', async () => {
      renderComponent()

      const textarea = getTextarea()

      // First trigger autocomplete with '@' at the end
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Continue typing 'co' to filter
      fireEvent.change(textarea, {
        target: { value: '@co', selectionStart: 3 }
      })

      // Autocomplete should still be visible
      expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()

      // Backspace to delete 'co' and '@'
      fireEvent.change(textarea, {
        target: { value: '', selectionStart: 0 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
      })
    })

    it('Escape dismisses autocomplete', async () => {
      renderComponent()

      const textarea = getTextarea()

      // First trigger autocomplete with '@'
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Type more to have query
      fireEvent.change(textarea, {
        target: { value: '@co', selectionStart: 3 }
      })

      // Autocomplete should still be visible
      expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()

      // Press Escape on textarea
      fireEvent.keyDown(textarea, { key: 'Escape' })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
      })
    })
  })

  describe('keyboard navigation in autocomplete', () => {
    it('ArrowUp/Down navigates autocomplete', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // First item should be highlighted by default
      let items = document.querySelectorAll('.mention-autocomplete-item')
      expect(items[0]).toHaveClass('highlighted')

      // Press down to highlight second item
      fireEvent.keyDown(document, { key: 'ArrowDown' })

      await waitFor(() => {
        items = document.querySelectorAll('.mention-autocomplete-item')
        expect(items[1]).toHaveClass('highlighted')
      })

      // Press up to go back to first item
      fireEvent.keyDown(document, { key: 'ArrowUp' })

      await waitFor(() => {
        items = document.querySelectorAll('.mention-autocomplete-item')
        expect(items[0]).toHaveClass('highlighted')
      })
    })

    it('Enter selects current autocomplete item', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Press Enter to select
      fireEvent.keyDown(document, { key: 'Enter' })

      await waitFor(() => {
        // Autocomplete should be dismissed
        expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
        // Text should contain the selected mention
        expect(textarea).toHaveValue('@coordinator ')
      })
    })

    it('Tab selects current autocomplete item', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Press Tab to select
      fireEvent.keyDown(document, { key: 'Tab' })

      await waitFor(() => {
        // Autocomplete should be dismissed
        expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
        // Text should contain the selected mention
        expect(textarea).toHaveValue('@coordinator ')
      })
    })

    it('Enter does NOT submit when autocomplete is active', async () => {
      const onSend = vi.fn().mockResolvedValue(undefined)
      renderComponent({ onSend })

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Press Enter - should select mention, not submit
      fireEvent.keyDown(document, { key: 'Enter' })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
      })

      // onSend should NOT have been called
      expect(onSend).not.toHaveBeenCalled()
    })
  })

  describe('completeMention', () => {
    it('replaces @query with @agentId + space', async () => {
      renderComponent()

      const textarea = getTextarea()

      // First trigger autocomplete with '@'
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Type more to filter (coord)
      fireEvent.change(textarea, {
        target: { value: '@coord', selectionStart: 6 }
      })

      // Select coordinator
      fireEvent.keyDown(document, { key: 'Enter' })

      await waitFor(() => {
        expect(textarea).toHaveValue('@coordinator ')
      })
    })

    it('preserves text before @', async () => {
      renderComponent()

      const textarea = getTextarea()

      // First trigger autocomplete with 'Hello @'
      fireEvent.change(textarea, {
        target: { value: 'Hello @', selectionStart: 7 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Type more to filter (coord)
      fireEvent.change(textarea, {
        target: { value: 'Hello @coord', selectionStart: 12 }
      })

      // Select coordinator
      fireEvent.keyDown(document, { key: 'Enter' })

      await waitFor(() => {
        expect(textarea).toHaveValue('Hello @coordinator ')
      })
    })

    it('focus returns to textarea after completion', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })

      // Select an item
      fireEvent.keyDown(document, { key: 'Enter' })

      // Use act to wait for requestAnimationFrame
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 50))
      })

      expect(document.activeElement).toBe(textarea)
    })
  })

  describe('aria-expanded attribute', () => {
    it('sets aria-expanded when autocomplete is open', async () => {
      renderComponent()

      const textarea = getTextarea()
      expect(textarea).toHaveAttribute('aria-expanded', 'false')

      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(textarea).toHaveAttribute('aria-expanded', 'true')
      })
    })

    it('removes aria-expanded when autocomplete is closed', async () => {
      renderComponent()

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      await waitFor(() => {
        expect(textarea).toHaveAttribute('aria-expanded', 'true')
      })

      // Dismiss with Escape
      fireEvent.keyDown(textarea, { key: 'Escape' })

      await waitFor(() => {
        expect(textarea).toHaveAttribute('aria-expanded', 'false')
      })
    })
  })

  describe('no agentIds provided', () => {
    it('does not show autocomplete when agentIds is empty', async () => {
      renderComponent({ agentIds: [] })

      const textarea = getTextarea()
      fireEvent.change(textarea, {
        target: { value: '@', selectionStart: 1 }
      })

      // MentionAutocomplete should NOT appear
      expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
    })
  })
})
