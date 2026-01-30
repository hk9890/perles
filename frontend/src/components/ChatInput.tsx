import { useState, useRef, useCallback, type KeyboardEvent, type ChangeEvent } from 'react'
import MentionAutocomplete from './MentionAutocomplete'
import './ChatInput.css'

const MAX_CHARS = 10000
const CHAR_COUNTER_THRESHOLD = 8000
const CHAR_WARNING_THRESHOLD = 9500

interface MentionState {
  isActive: boolean
  triggerPosition: number    // Cursor position where @ was typed
  query: string              // Text after @ for filtering
  cursorIndex: number        // Selected item in autocomplete list
}

interface ChatInputProps {
  channelSlug: string           // Target channel for new messages
  threadId?: string             // If set, message is a reply
  placeholder?: string          // Input placeholder text
  onSend: (content: string, mentions: string[]) => Promise<void>
  disabled?: boolean            // Disable when session is archived
  disabledReason?: string       // e.g., "This session has ended"
  agentIds?: string[]           // Available agents for @mention autocomplete
}

/**
 * Extract @mentions from content using regex.
 * Matches @username patterns where username starts with word char
 * and can contain word chars and hyphens.
 */
function extractMentions(content: string): string[] {
  const regex = /@(\w[\w-]*)/g
  const mentions: string[] = []
  let match
  while ((match = regex.exec(content)) !== null) {
    if (!mentions.includes(match[1])) {
      mentions.push(match[1])
    }
  }
  return mentions
}

export default function ChatInput({
  channelSlug,
  threadId,
  placeholder = 'Type a message...',
  onSend,
  disabled = false,
  disabledReason = 'This session has ended',
  agentIds = [],
}: ChatInputProps) {
  const [value, setValue] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [mentionState, setMentionState] = useState<MentionState | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const isInputDisabled = disabled || isSending
  const isSubmitDisabled = isInputDisabled || value.trim().length === 0 || value.length > MAX_CHARS
  const showCharCounter = value.length > CHAR_COUNTER_THRESHOLD
  const isOverLimit = value.length > MAX_CHARS
  const isNearLimit = value.length > CHAR_WARNING_THRESHOLD && value.length <= MAX_CHARS

  const handleChange = useCallback((e: ChangeEvent<HTMLTextAreaElement>) => {
    let newValue = e.target.value
    let cursorPos = e.target.selectionStart || 0

    // Enforce character limit by truncating input
    if (newValue.length > MAX_CHARS) {
      newValue = newValue.slice(0, MAX_CHARS)
      cursorPos = Math.min(cursorPos, MAX_CHARS)
    }

    setValue(newValue)

    // If mention autocomplete is not active, check for @ trigger
    if (!mentionState) {
      const textBeforeCursor = newValue.slice(0, cursorPos)
      // Check if @ was just typed after whitespace or at start
      if (/(?:^|\s)@$/.test(textBeforeCursor)) {
        setMentionState({
          isActive: true,
          triggerPosition: cursorPos - 1,
          query: '',
          cursorIndex: 0
        })
      }
    } else {
      // Mention autocomplete is active - update query or dismiss
      const query = newValue.slice(mentionState.triggerPosition + 1, cursorPos)
      // Dismiss if space typed, cursor moved before @, or @ was deleted
      if (/\s/.test(query) || cursorPos <= mentionState.triggerPosition || newValue[mentionState.triggerPosition] !== '@') {
        setMentionState(null)
      } else {
        setMentionState(prev => prev ? { ...prev, query, cursorIndex: 0 } : null)
      }
    }
  }, [mentionState])

  // Complete mention by replacing @query with @agentId
  const completeMention = useCallback((agentId: string) => {
    if (!mentionState) return

    const before = value.slice(0, mentionState.triggerPosition)
    const cursorPos = textareaRef.current?.selectionStart || value.length
    const after = value.slice(cursorPos)
    const newValue = `${before}@${agentId} ${after}`
    setValue(newValue)
    setMentionState(null)

    // Restore focus to textarea and set cursor position after inserted mention
    requestAnimationFrame(() => {
      if (textareaRef.current) {
        textareaRef.current.focus()
        const newCursorPos = before.length + agentId.length + 2 // +2 for @ and space
        textareaRef.current.setSelectionRange(newCursorPos, newCursorPos)
      }
    })
  }, [mentionState, value])

  // Handle navigation in mention autocomplete
  const handleMentionNavigate = useCallback((direction: 'up' | 'down') => {
    if (!mentionState) return

    // Filter agents to get count for wrapping
    const filteredCount = agentIds.filter(id =>
      !mentionState.query || id.toLowerCase().includes(mentionState.query.toLowerCase())
    ).length
    if (filteredCount === 0) return

    setMentionState(prev => {
      if (!prev) return null
      let newIndex = prev.cursorIndex
      if (direction === 'up') {
        newIndex = prev.cursorIndex <= 0 ? filteredCount - 1 : prev.cursorIndex - 1
      } else {
        newIndex = prev.cursorIndex >= filteredCount - 1 ? 0 : prev.cursorIndex + 1
      }
      return { ...prev, cursorIndex: newIndex }
    })
  }, [mentionState, agentIds])

  const handleSubmit = useCallback(async () => {
    const trimmed = value.trim()
    if (!trimmed || isSending || disabled || trimmed.length > MAX_CHARS) {
      return
    }

    const mentions = extractMentions(trimmed)
    setIsSending(true)

    try {
      await onSend(trimmed, mentions)
      // Clear on success
      setValue('')
    } catch {
      // Preserve content on error - user can retry
    } finally {
      setIsSending(false)
      // Restore focus to textarea
      textareaRef.current?.focus()
    }
  }, [value, isSending, disabled, onSend])

  const handleKeyDown = useCallback((e: KeyboardEvent<HTMLTextAreaElement>) => {
    // When mention autocomplete is active, let MentionAutocomplete handle navigation keys
    // MentionAutocomplete captures these keys in document-level event listener
    if (mentionState?.isActive) {
      // Enter without shift should not submit when autocomplete is active
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        // MentionAutocomplete will handle the Enter key
        return
      }
      // Escape dismisses autocomplete
      if (e.key === 'Escape') {
        e.preventDefault()
        setMentionState(null)
        return
      }
    } else {
      // Enter without Shift submits when autocomplete is not active
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        handleSubmit()
      }
    }
    // Shift+Enter inserts newline (default behavior, no preventDefault needed)
  }, [handleSubmit, mentionState])

  // Get anchor rect for positioning the autocomplete
  const getAnchorRect = (): DOMRect | null => {
    return textareaRef.current?.getBoundingClientRect() ?? null
  }

  return (
    <div className={`chat-input ${disabled ? 'disabled' : ''}`}>
      {disabled && (
        <div className="chat-input-disabled-overlay">
          <span>{disabledReason}</span>
        </div>
      )}

      <div className="chat-input-container">
        <textarea
          ref={textareaRef}
          className="chat-input-textarea"
          value={value}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={isInputDisabled}
          rows={1}
          aria-label={threadId ? `Reply to thread in #${channelSlug}` : `Message #${channelSlug}`}
          aria-expanded={mentionState?.isActive ?? false}
          aria-haspopup="listbox"
          aria-controls={mentionState?.isActive ? 'mention-autocomplete-listbox' : undefined}
          aria-autocomplete="list"
        />

        <div className="chat-input-actions">
          {showCharCounter && (
            <span className={`chat-input-char-counter ${isOverLimit ? 'over-limit' : ''} ${isNearLimit ? 'near-limit' : ''}`}>
              {value.length}/{MAX_CHARS}
            </span>
          )}

          <button
            className="chat-input-send-btn"
            onClick={handleSubmit}
            disabled={isSubmitDisabled}
            aria-label="Send message"
          >
            {isSending ? (
              <span className="chat-input-spinner" />
            ) : (
              <svg viewBox="0 0 20 20" fill="currentColor" width="18" height="18">
                <path d="M10.894 2.553a1 1 0 00-1.788 0l-7 14a1 1 0 001.169 1.409l5-1.429A1 1 0 009 15.571V11a1 1 0 112 0v4.571a1 1 0 00.725.962l5 1.428a1 1 0 001.17-1.408l-7-14z" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {mentionState?.isActive && agentIds.length > 0 && (
        <MentionAutocomplete
          query={mentionState.query}
          agentIds={agentIds}
          cursorIndex={mentionState.cursorIndex}
          onSelect={completeMention}
          onDismiss={() => setMentionState(null)}
          onNavigate={handleMentionNavigate}
          anchorRect={getAnchorRect()}
        />
      )}
    </div>
  )
}
