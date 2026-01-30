import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import MentionAutocomplete, { MentionAutocompleteProps } from './MentionAutocomplete'

// Mock anchorRect for positioning
const mockAnchorRect: DOMRect = {
  top: 100,
  left: 50,
  bottom: 120,
  right: 150,
  width: 100,
  height: 20,
  x: 50,
  y: 100,
  toJSON: () => ({}),
}

const defaultAgents = ['coordinator', 'worker-1', 'worker-2', 'worker-3']

function renderComponent(props: Partial<MentionAutocompleteProps> = {}) {
  const defaultProps: MentionAutocompleteProps = {
    query: '',
    agentIds: defaultAgents,
    cursorIndex: 0,
    onSelect: vi.fn(),
    onDismiss: vi.fn(),
    onNavigate: vi.fn(),
    anchorRect: mockAnchorRect,
  }
  return render(<MentionAutocomplete {...defaultProps} {...props} />)
}

describe('MentionAutocomplete', () => {
  beforeEach(() => {
    // Reset window.innerHeight for position calculations
    Object.defineProperty(window, 'innerHeight', { value: 768, writable: true })
  })

  afterEach(() => {
    cleanup()
  })

  describe('rendering', () => {
    it('renders as a React Portal to document.body', () => {
      renderComponent()
      // Portal content should be at document.body level, not inside test container
      const autocomplete = document.querySelector('.mention-autocomplete')
      expect(autocomplete).toBeInTheDocument()
      expect(autocomplete?.parentElement).toBe(document.body)
    })

    it('renders all agents when query is empty', () => {
      renderComponent({ query: '' })
      expect(screen.getByText('@coordinator')).toBeInTheDocument()
      expect(screen.getByText('@worker-1')).toBeInTheDocument()
      expect(screen.getByText('@worker-2')).toBeInTheDocument()
      expect(screen.getByText('@worker-3')).toBeInTheDocument()
    })

    it('renders nothing when anchorRect is null', () => {
      renderComponent({ anchorRect: null })
      const autocomplete = document.querySelector('.mention-autocomplete')
      expect(autocomplete).not.toBeInTheDocument()
    })

    it('renders with correct ARIA attributes', () => {
      renderComponent()
      const listbox = screen.getByRole('listbox')
      expect(listbox).toBeInTheDocument()
      expect(listbox).toHaveAttribute('aria-label', 'Agent mentions')
      expect(listbox).toHaveAttribute('id', 'mention-autocomplete-listbox')

      const options = screen.getAllByRole('option')
      expect(options).toHaveLength(4)
      // Each option should have an id for aria-activedescendant
      expect(options[0]).toHaveAttribute('id', 'mention-option-coordinator')
      expect(options[1]).toHaveAttribute('id', 'mention-option-worker-1')
    })

    it('sets aria-activedescendant on listbox', () => {
      renderComponent({ cursorIndex: 1 })
      const listbox = screen.getByRole('listbox')
      expect(listbox).toHaveAttribute('aria-activedescendant', 'mention-option-worker-1')
    })

    it('empty state has correct ARIA attributes', () => {
      renderComponent({ query: 'nonexistent' })
      const emptyState = screen.getByText('No matching agents')
      expect(emptyState).toHaveAttribute('role', 'status')
      expect(emptyState).toHaveAttribute('aria-live', 'polite')
    })

    it('positions above the input anchor', () => {
      renderComponent()
      const autocomplete = document.querySelector('.mention-autocomplete') as HTMLElement
      expect(autocomplete.style.position).toBe('fixed')
      expect(autocomplete.style.left).toBe('50px')
      // Bottom position: window.innerHeight (768) - anchorRect.top (100) + 4 = 672px
      expect(autocomplete.style.bottom).toBe('672px')
    })
  })

  describe('filtering', () => {
    it('filters agents case-insensitively', () => {
      renderComponent({ query: 'COORD' })
      expect(screen.getByText('@coordinator')).toBeInTheDocument()
      expect(screen.queryByText('@worker-1')).not.toBeInTheDocument()
    })

    it('filters agents with partial match', () => {
      renderComponent({ query: 'work' })
      expect(screen.queryByText('@coordinator')).not.toBeInTheDocument()
      expect(screen.getByText('@worker-1')).toBeInTheDocument()
      expect(screen.getByText('@worker-2')).toBeInTheDocument()
      expect(screen.getByText('@worker-3')).toBeInTheDocument()
    })

    it('filters by number in worker ID', () => {
      renderComponent({ query: '2' })
      expect(screen.queryByText('@coordinator')).not.toBeInTheDocument()
      expect(screen.queryByText('@worker-1')).not.toBeInTheDocument()
      expect(screen.getByText('@worker-2')).toBeInTheDocument()
      expect(screen.queryByText('@worker-3')).not.toBeInTheDocument()
    })

    it('shows "No matching agents" when filter has no results', () => {
      renderComponent({ query: 'nonexistent' })
      expect(screen.getByText('No matching agents')).toBeInTheDocument()
    })
  })

  describe('highlighting', () => {
    it('highlights the item at cursorIndex', () => {
      renderComponent({ cursorIndex: 1 })
      const items = document.querySelectorAll('.mention-autocomplete-item')
      expect(items[0]).not.toHaveClass('highlighted')
      expect(items[1]).toHaveClass('highlighted')
      expect(items[2]).not.toHaveClass('highlighted')
    })

    it('sets aria-selected on highlighted item', () => {
      renderComponent({ cursorIndex: 2 })
      const options = screen.getAllByRole('option')
      expect(options[0]).toHaveAttribute('aria-selected', 'false')
      expect(options[1]).toHaveAttribute('aria-selected', 'false')
      expect(options[2]).toHaveAttribute('aria-selected', 'true')
      expect(options[3]).toHaveAttribute('aria-selected', 'false')
    })

    it('clamps cursorIndex to valid range', () => {
      renderComponent({ cursorIndex: 100 })
      // Should highlight the last item when index is out of bounds
      const items = document.querySelectorAll('.mention-autocomplete-item')
      expect(items[3]).toHaveClass('highlighted')
    })

    it('handles negative cursorIndex', () => {
      renderComponent({ cursorIndex: -5 })
      // Should highlight the first item when index is negative
      const items = document.querySelectorAll('.mention-autocomplete-item')
      expect(items[0]).toHaveClass('highlighted')
    })
  })

  describe('keyboard navigation', () => {
    it('calls onNavigate("up") on ArrowUp', () => {
      const onNavigate = vi.fn()
      renderComponent({ onNavigate })

      fireEvent.keyDown(document, { key: 'ArrowUp' })
      expect(onNavigate).toHaveBeenCalledWith('up')
    })

    it('calls onNavigate("down") on ArrowDown', () => {
      const onNavigate = vi.fn()
      renderComponent({ onNavigate })

      fireEvent.keyDown(document, { key: 'ArrowDown' })
      expect(onNavigate).toHaveBeenCalledWith('down')
    })

    it('calls onSelect with correct agent on Enter', () => {
      const onSelect = vi.fn()
      renderComponent({ onSelect, cursorIndex: 1 })

      fireEvent.keyDown(document, { key: 'Enter' })
      expect(onSelect).toHaveBeenCalledWith('worker-1')
    })

    it('calls onSelect with correct agent on Tab', () => {
      const onSelect = vi.fn()
      renderComponent({ onSelect, cursorIndex: 2 })

      fireEvent.keyDown(document, { key: 'Tab' })
      expect(onSelect).toHaveBeenCalledWith('worker-2')
    })

    it('calls onDismiss on Escape', () => {
      const onDismiss = vi.fn()
      renderComponent({ onDismiss })

      fireEvent.keyDown(document, { key: 'Escape' })
      expect(onDismiss).toHaveBeenCalled()
    })

    it('prevents default and stops propagation for handled keys', () => {
      renderComponent()

      const event = new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })
      const preventDefaultSpy = vi.spyOn(event, 'preventDefault')
      const stopPropagationSpy = vi.spyOn(event, 'stopPropagation')

      document.dispatchEvent(event)

      expect(preventDefaultSpy).toHaveBeenCalled()
      expect(stopPropagationSpy).toHaveBeenCalled()
    })

    it('does not call onSelect when no agents match filter', () => {
      const onSelect = vi.fn()
      renderComponent({ query: 'nonexistent', onSelect })

      fireEvent.keyDown(document, { key: 'Enter' })
      expect(onSelect).not.toHaveBeenCalled()
    })

    it('selects correct filtered agent when filter is active', () => {
      const onSelect = vi.fn()
      renderComponent({ query: 'worker', onSelect, cursorIndex: 1 })

      // With 'worker' filter, agents are [worker-1, worker-2, worker-3]
      // cursorIndex 1 = worker-2
      fireEvent.keyDown(document, { key: 'Enter' })
      expect(onSelect).toHaveBeenCalledWith('worker-2')
    })
  })

  describe('click interactions', () => {
    it('calls onSelect when clicking an agent', () => {
      const onSelect = vi.fn()
      renderComponent({ onSelect })

      fireEvent.click(screen.getByText('@worker-2'))
      expect(onSelect).toHaveBeenCalledWith('worker-2')
    })

    it('calls onDismiss when clicking outside', () => {
      const onDismiss = vi.fn()
      renderComponent({ onDismiss })

      fireEvent.mouseDown(document.body)
      expect(onDismiss).toHaveBeenCalled()
    })

    it('does not call onDismiss when clicking inside autocomplete', () => {
      const onDismiss = vi.fn()
      renderComponent({ onDismiss })

      const autocomplete = document.querySelector('.mention-autocomplete')!
      fireEvent.mouseDown(autocomplete)
      expect(onDismiss).not.toHaveBeenCalled()
    })
  })

  describe('mouse hover', () => {
    it('calls onNavigate when hovering over different item', () => {
      const onNavigate = vi.fn()
      renderComponent({ onNavigate, cursorIndex: 0 })

      // Hover over third item (index 2), should navigate down twice
      const items = document.querySelectorAll('.mention-autocomplete-item')
      fireEvent.mouseEnter(items[2])

      // Should call onNavigate('down') twice to go from 0 to 2
      expect(onNavigate).toHaveBeenCalledTimes(2)
      expect(onNavigate).toHaveBeenCalledWith('down')
    })

    it('navigates up when hovering over earlier item', () => {
      const onNavigate = vi.fn()
      renderComponent({ onNavigate, cursorIndex: 3 })

      // Hover over first item (index 0), should navigate up 3 times
      const items = document.querySelectorAll('.mention-autocomplete-item')
      fireEvent.mouseEnter(items[0])

      expect(onNavigate).toHaveBeenCalledTimes(3)
      expect(onNavigate).toHaveBeenCalledWith('up')
    })

    it('does not navigate when hovering over currently highlighted item', () => {
      const onNavigate = vi.fn()
      renderComponent({ onNavigate, cursorIndex: 1 })

      const items = document.querySelectorAll('.mention-autocomplete-item')
      fireEvent.mouseEnter(items[1])

      expect(onNavigate).not.toHaveBeenCalled()
    })
  })

  describe('avatar display', () => {
    it('displays avatar with initials for each agent', () => {
      renderComponent()

      const avatars = document.querySelectorAll('.mention-autocomplete-avatar')
      expect(avatars).toHaveLength(4)

      // Coordinator shows 'C'
      expect(avatars[0]).toHaveTextContent('C')
      // Workers show first letter + number
      expect(avatars[1]).toHaveTextContent('W1')
      expect(avatars[2]).toHaveTextContent('W2')
      expect(avatars[3]).toHaveTextContent('W3')
    })
  })

  describe('cleanup', () => {
    it('removes event listeners on unmount', () => {
      const { unmount } = renderComponent()
      const removeEventListenerSpy = vi.spyOn(document, 'removeEventListener')

      unmount()

      expect(removeEventListenerSpy).toHaveBeenCalled()
      removeEventListenerSpy.mockRestore()
    })
  })
})
