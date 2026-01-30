import { useEffect, useMemo, useRef } from 'react'
import { createPortal } from 'react-dom'
import { hashColor, getInitials } from '../utils/colors'
import './MentionAutocomplete.css'

export interface MentionAutocompleteProps {
  query: string                 // Filter text after @
  agentIds: string[]            // Available agents (coordinator + workers)
  cursorIndex: number           // Currently highlighted item
  onSelect: (agentId: string) => void
  onDismiss: () => void
  onNavigate: (direction: 'up' | 'down') => void
  anchorRect: DOMRect | null    // Position reference from textarea
}

export default function MentionAutocomplete({
  query,
  agentIds,
  cursorIndex,
  onSelect,
  onDismiss,
  onNavigate,
  anchorRect,
}: MentionAutocompleteProps) {
  const listRef = useRef<HTMLUListElement>(null)

  // Filter agents by query (case-insensitive)
  const filteredAgents = useMemo(() => {
    if (!query) return agentIds
    const lowerQuery = query.toLowerCase()
    return agentIds.filter(id => id.toLowerCase().includes(lowerQuery))
  }, [agentIds, query])

  // Ensure cursor stays in bounds
  const safeCursorIndex = Math.max(0, Math.min(cursorIndex, filteredAgents.length - 1))

  // Scroll highlighted item into view
  useEffect(() => {
    if (listRef.current && filteredAgents.length > 0) {
      const highlightedItem = listRef.current.children[safeCursorIndex] as HTMLElement
      if (highlightedItem) {
        highlightedItem.scrollIntoView({ block: 'nearest' })
      }
    }
  }, [safeCursorIndex, filteredAgents.length])

  // Handle keyboard events
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'ArrowUp':
          e.preventDefault()
          e.stopPropagation()
          onNavigate('up')
          break
        case 'ArrowDown':
          e.preventDefault()
          e.stopPropagation()
          onNavigate('down')
          break
        case 'Enter':
        case 'Tab':
          e.preventDefault()
          e.stopPropagation()
          if (filteredAgents.length > 0) {
            onSelect(filteredAgents[safeCursorIndex])
          }
          break
        case 'Escape':
          e.preventDefault()
          e.stopPropagation()
          onDismiss()
          break
      }
    }

    // Capture phase to intercept before textarea
    document.addEventListener('keydown', handleKeyDown, true)
    return () => document.removeEventListener('keydown', handleKeyDown, true)
  }, [filteredAgents, safeCursorIndex, onSelect, onDismiss, onNavigate])

  // Handle click outside to dismiss
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as HTMLElement
      if (!target.closest('.mention-autocomplete')) {
        onDismiss()
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [onDismiss])

  // Don't render if no anchor position
  if (!anchorRect) return null

  // Calculate position (above the input)
  const style: React.CSSProperties = {
    position: 'fixed',
    left: anchorRect.left,
    bottom: window.innerHeight - anchorRect.top + 4,
  }

  // Generate unique option IDs for aria-activedescendant
  const getOptionId = (agentId: string) => `mention-option-${agentId}`
  const activeDescendantId = filteredAgents.length > 0 ? getOptionId(filteredAgents[safeCursorIndex]) : undefined

  const content = (
    <div className="mention-autocomplete" style={style} role="presentation">
      {filteredAgents.length === 0 ? (
        <div className="mention-autocomplete-empty" role="status" aria-live="polite">No matching agents</div>
      ) : (
        <ul
          ref={listRef}
          className="mention-autocomplete-list"
          role="listbox"
          id="mention-autocomplete-listbox"
          aria-label="Agent mentions"
          aria-activedescendant={activeDescendantId}
        >
          {filteredAgents.map((agentId, index) => (
            <li
              key={agentId}
              id={getOptionId(agentId)}
              role="option"
              aria-selected={index === safeCursorIndex}
              className={`mention-autocomplete-item ${index === safeCursorIndex ? 'highlighted' : ''}`}
              onClick={() => onSelect(agentId)}
              onMouseEnter={() => {
                // Update cursor on hover
                if (index !== safeCursorIndex) {
                  // Navigate to match index
                  const diff = index - safeCursorIndex
                  if (diff > 0) {
                    for (let i = 0; i < diff; i++) onNavigate('down')
                  } else {
                    for (let i = 0; i < -diff; i++) onNavigate('up')
                  }
                }
              }}
            >
              <div
                className="mention-autocomplete-avatar"
                style={{ background: hashColor(agentId) }}
              >
                {getInitials(agentId)}
              </div>
              <span className="mention-autocomplete-name">@{agentId}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )

  // Render as portal to document.body
  return createPortal(content, document.body)
}
