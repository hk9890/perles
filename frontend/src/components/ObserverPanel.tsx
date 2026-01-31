import { useMemo } from 'react'
import type { AgentMessage } from '../types'
import Markdown from './Markdown'
import './ObserverPanel.css'

interface Props {
  messages: AgentMessage[]
  notes: string
}

interface MessageGroup {
  role: string
  ts: string
  isToolGroup: boolean
  messages: AgentMessage[]
}

const roleColors: Record<string, string> = {
  'observer': 'var(--accent-purple)',
  'system': 'var(--text-muted)',
}

export default function ObserverPanel({ messages, notes }: Props) {
  const formatTime = (ts: string) => {
    return new Date(ts).toLocaleTimeString()
  }

  // Group consecutive tool calls from the same role
  const groupedMessages = useMemo(() => {
    const groups: MessageGroup[] = []
    
    for (const msg of messages) {
      const lastGroup = groups[groups.length - 1]
      
      // If this is a tool call and the last group is tool calls from same role, add to it
      if (
        msg.is_tool_call &&
        lastGroup?.isToolGroup &&
        lastGroup.role === msg.role
      ) {
        lastGroup.messages.push(msg)
      } else {
        // Start a new group
        groups.push({
          role: msg.role,
          ts: msg.ts,
          isToolGroup: !!msg.is_tool_call,
          messages: [msg],
        })
      }
    }
    
    return groups
  }, [messages])

  return (
    <div className="observer-panel">
      <div className="observer-split">
        {/* Left side: Messages */}
        <div className="observer-messages">
          <h3>Observer Messages ({messages.length})</h3>
          <div className="messages-list">
            {groupedMessages.map((group, index) => (
              <div 
                key={index} 
                className={`message-item ${group.isToolGroup ? 'tool-call' : ''}`}
              >
                <div className="message-header">
                  <span 
                    className="message-role"
                    style={{ color: roleColors[group.role] || 'var(--text-secondary)' }}
                  >
                    {group.role}
                  </span>
                  <span className="message-time">{formatTime(group.ts)}</span>
                </div>
                
                <div className="message-content">
                  {group.isToolGroup ? (
                    <div className="tool-group">
                      {group.messages.map((msg, i) => (
                        <span key={i} className="tool-badge">{msg.content}</span>
                      ))}
                    </div>
                  ) : (
                    <pre>{group.messages[0].content}</pre>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Right side: Notes */}
        <div className="observer-notes">
          <h3>Observer Notes</h3>
          <div className="notes-content">
            {notes ? (
              <Markdown content={notes} className="markdown-compact" />
            ) : (
              <p className="empty-notes">No notes recorded</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
