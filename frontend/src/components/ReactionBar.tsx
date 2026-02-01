import { useState, useRef, useEffect } from 'react'
import './ReactionBar.css'

export interface ReactionSummary {
  emoji: string
  count: number
  agentIds: string[]
}

export interface ReactionBarProps {
  messageId: string
  reactions: ReactionSummary[]
  onReact: (emoji: string, remove: boolean) => Promise<void>
  disabled?: boolean
}

// Quick reaction emojis (most commonly used)
const QUICK_EMOJIS = ['👍', '❤️', '👀', '🎉', '🚀', '✅']

// Full emoji picker categories
const EMOJI_CATEGORIES = {
  'Smileys': ['😀', '😃', '😄', '😁', '😅', '😂', '🤣', '😊', '😇', '🙂', '😉', '😌', '😍', '🥰', '😘'],
  'Gestures': ['👍', '👎', '👏', '🙌', '🤝', '🙏', '💪', '✌️', '🤞', '🤟', '👋', '🤙', '👊', '✊'],
  'Symbols': ['❤️', '🧡', '💛', '💚', '💙', '💜', '💔', '❣️', '💕', '💖', '💗', '💘', '💝', '⭐', '✨'],
  'Objects': ['🎉', '🎊', '🎁', '🏆', '🥇', '🎯', '🚀', '💡', '🔥', '✅', '❌', '⚠️', '📌', '📝', '💬'],
  'Nature': ['🌟', '🌙', '☀️', '🌈', '⚡', '🌊', '🌸', '🌺', '🌻', '🍀', '🌲', '🌴', '🦋', '🐝', '🐞'],
}

export default function ReactionBar({
  messageId,
  reactions,
  onReact,
  disabled = false,
}: ReactionBarProps) {
  const [showPicker, setShowPicker] = useState(false)
  const [selectedCategory, setSelectedCategory] = useState<keyof typeof EMOJI_CATEGORIES>('Smileys')
  const [isLoading, setIsLoading] = useState<string | null>(null)
  const pickerRef = useRef<HTMLDivElement>(null)

  // Close picker on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (pickerRef.current && !pickerRef.current.contains(e.target as Node)) {
        setShowPicker(false)
      }
    }

    if (showPicker) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showPicker])

  // Check if user has reacted with this emoji
  const hasUserReaction = (emoji: string): boolean => {
    const reaction = reactions.find(r => r.emoji === emoji)
    return reaction?.agentIds.includes('user') ?? false
  }

  const handleReaction = async (emoji: string) => {
    if (disabled || isLoading) return

    const remove = hasUserReaction(emoji)
    setIsLoading(emoji)

    try {
      await onReact(emoji, remove)
      setShowPicker(false)
    } finally {
      setIsLoading(null)
    }
  }

  return (
    <div className="reaction-bar" data-message-id={messageId}>
      {/* Quick emoji buttons */}
      <div className="reaction-bar-quick">
        {QUICK_EMOJIS.map((emoji) => {
          const isActive = hasUserReaction(emoji)
          const isLoadingThis = isLoading === emoji

          return (
            <button
              key={emoji}
              className={`reaction-bar-btn ${isActive ? 'active' : ''} ${isLoadingThis ? 'loading' : ''}`}
              onClick={() => handleReaction(emoji)}
              disabled={disabled || isLoading !== null}
              title={isActive ? `Remove ${emoji}` : `React with ${emoji}`}
            >
              {emoji}
            </button>
          )
        })}

        {/* Emoji picker toggle */}
        <button
          className={`reaction-bar-btn picker-toggle ${showPicker ? 'active' : ''}`}
          onClick={() => setShowPicker(!showPicker)}
          disabled={disabled}
          title="More reactions"
        >
          <span className="picker-icon">+</span>
        </button>
      </div>

      {/* Emoji picker popover */}
      {showPicker && (
        <div className="reaction-picker" ref={pickerRef}>
          {/* Category tabs */}
          <div className="reaction-picker-tabs">
            {(Object.keys(EMOJI_CATEGORIES) as (keyof typeof EMOJI_CATEGORIES)[]).map((category) => (
              <button
                key={category}
                className={`picker-tab ${selectedCategory === category ? 'active' : ''}`}
                onClick={() => setSelectedCategory(category)}
              >
                {category}
              </button>
            ))}
          </div>

          {/* Emoji grid */}
          <div className="reaction-picker-grid">
            {EMOJI_CATEGORIES[selectedCategory].map((emoji) => {
              const isActive = hasUserReaction(emoji)
              const isLoadingThis = isLoading === emoji

              return (
                <button
                  key={emoji}
                  className={`picker-emoji ${isActive ? 'active' : ''} ${isLoadingThis ? 'loading' : ''}`}
                  onClick={() => handleReaction(emoji)}
                  disabled={isLoading !== null}
                >
                  {emoji}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
