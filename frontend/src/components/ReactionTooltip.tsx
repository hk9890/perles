import { useState, useRef, useEffect, cloneElement, isValidElement } from 'react'
import { createPortal } from 'react-dom'

interface ReactionTooltipProps {
  emoji: string
  names: string
  children: React.ReactElement
}

export default function ReactionTooltip({ emoji, names, children }: ReactionTooltipProps) {
  const [isVisible, setIsVisible] = useState(false)
  const [position, setPosition] = useState({ top: 0, left: 0 })
  const triggerRef = useRef<HTMLElement>(null)

  useEffect(() => {
    if (isVisible && triggerRef.current) {
      const rect = triggerRef.current.getBoundingClientRect()
      setPosition({
        top: rect.top - 8,
        left: rect.left + rect.width / 2,
      })
    }
  }, [isVisible])

  // Clone the child element to add event handlers and ref
  const trigger = isValidElement(children)
    ? cloneElement(children, {
        ref: triggerRef,
        onMouseEnter: (e: React.MouseEvent) => {
          setIsVisible(true)
          // Call original handler if exists
          const original = (children.props as Record<string, unknown>).onMouseEnter
          if (typeof original === 'function') original(e)
        },
        onMouseLeave: (e: React.MouseEvent) => {
          setIsVisible(false)
          const original = (children.props as Record<string, unknown>).onMouseLeave
          if (typeof original === 'function') original(e)
        },
      } as Partial<unknown>)
    : children

  return (
    <>
      {trigger}
      {isVisible && createPortal(
        <div
          className="reaction-tooltip-portal"
          style={{
            position: 'fixed',
            top: position.top,
            left: position.left,
            transform: 'translate(-50%, -100%)',
            zIndex: 99999,
          }}
        >
          <div className="reaction-tooltip-content">
            <span className="reaction-tooltip-emoji">{emoji}</span>
            <span className="reaction-tooltip-names">{names}</span>
          </div>
        </div>,
        document.body
      )}
    </>
  )
}
