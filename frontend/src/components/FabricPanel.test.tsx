import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import FabricPanel from './FabricPanel'
import type { FabricEvent, AgentsResponse } from '../types'

// Mock the global fetch
const mockFetch = vi.fn()
// eslint-disable-next-line @typescript-eslint/no-explicit-any
;(globalThis as any).fetch = mockFetch

// Helper to create fabric events for testing
function createChannelEvent(id: string, slug: string, title: string): FabricEvent {
  return {
    version: 1,
    timestamp: new Date().toISOString(),
    event: {
      type: 'channel.created',
      timestamp: new Date().toISOString(),
      channel_id: id,
      thread: {
        id,
        type: 'channel',
        created_at: new Date().toISOString(),
        created_by: 'system',
        slug,
        title,
        purpose: `${title} channel`,
        seq: 0,
      },
    },
  }
}

function createMessageEvent(
  channelId: string,
  messageId: string,
  createdBy: string,
  content: string,
  seq: number
): FabricEvent {
  return {
    version: 1,
    timestamp: new Date().toISOString(),
    event: {
      type: 'message.posted',
      timestamp: new Date().toISOString(),
      channel_id: channelId,
      thread: {
        id: messageId,
        type: 'message',
        created_at: new Date().toISOString(),
        created_by: createdBy,
        content,
        seq,
      },
    },
  }
}

const sampleEvents: FabricEvent[] = [
  createChannelEvent('ch-tasks', 'tasks', 'Tasks'),
  createChannelEvent('ch-general', 'general', 'General'),
  createMessageEvent('ch-tasks', 'msg-1', 'coordinator', 'Hello from coordinator', 1),
  createMessageEvent('ch-tasks', 'msg-2', 'worker-1', 'Task assigned', 2),
]

const mockAgentsResponse: AgentsResponse = {
  agents: [
    { id: 'coordinator', role: 'coordinator' },
    { id: 'worker-1', role: 'worker' },
    { id: 'worker-2', role: 'worker' },
  ],
  isActive: true,
}

describe('FabricPanel', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    // Default mock for agents endpoint
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/api/fabric/agents')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(mockAgentsResponse),
        })
      }
      if (url.includes('/api/fabric/send-message')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ success: true, messageId: 'new-msg-1' }),
        })
      }
      if (url.includes('/api/fabric/reply')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ success: true }),
        })
      }
      return Promise.reject(new Error('Unknown endpoint'))
    })
  })

  afterEach(() => {
    cleanup()
  })

  describe('rendering', () => {
    it('renders channel list from events', () => {
      render(<FabricPanel events={sampleEvents} />)
      // Use getAllByText since channel name appears in sidebar and header
      expect(screen.getAllByText('Tasks').length).toBeGreaterThan(0)
      expect(screen.getByText('General')).toBeInTheDocument()
    })

    it('renders messages in selected channel', () => {
      render(<FabricPanel events={sampleEvents} />)
      expect(screen.getByText('Hello from coordinator')).toBeInTheDocument()
      expect(screen.getByText('Task assigned')).toBeInTheDocument()
    })

    it('renders ChatInput at bottom of channel view', () => {
      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)
      // ChatInput should render with placeholder
      expect(screen.getByPlaceholderText('Message #Tasks...')).toBeInTheDocument()
    })
  })

  describe('agents fetching', () => {
    it('fetches agents when workflowId is provided', async () => {
      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/api/fabric/agents?workflowId=test-workflow')
        )
      })
    })

    it('does not fetch agents when workflowId is not provided', async () => {
      render(<FabricPanel events={sampleEvents} />)

      // Wait a bit to ensure no fetch is made
      await new Promise(resolve => setTimeout(resolve, 50))
      expect(mockFetch).not.toHaveBeenCalled()
    })

    it('updates isWorkflowActive from agents response', async () => {
      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      await waitFor(() => {
        // ChatInput should be enabled when isWorkflowActive is true
        const input = screen.getByPlaceholderText('Message #Tasks...')
        expect(input).not.toBeDisabled()
      })
    })

    it('disables ChatInput when isWorkflowActive is false', async () => {
      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/fabric/agents')) {
          return Promise.resolve({
            ok: true,
            json: () =>
              Promise.resolve({
                agents: [],
                isActive: false,
              }),
          })
        }
        return Promise.reject(new Error('Unknown endpoint'))
      })

      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      await waitFor(() => {
        const input = screen.getByPlaceholderText('Message #Tasks...')
        expect(input).toBeDisabled()
      })
    })
  })

  describe('send handlers', () => {
    it('handleChannelSend calls /api/fabric/send-message with correct params', async () => {
      const user = userEvent.setup()

      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      // Wait for agents to load
      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/api/fabric/agents')
        )
      })

      const input = screen.getByPlaceholderText('Message #Tasks...')
      // Type message that will trigger autocomplete at @
      await user.type(input, 'Hello world @')

      // Autocomplete should appear - select coordinator with Enter
      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).toBeInTheDocument()
      })
      await user.keyboard('{Enter}')

      // Autocomplete should be dismissed after selection
      await waitFor(() => {
        expect(document.querySelector('.mention-autocomplete')).not.toBeInTheDocument()
      })

      // Now submit the message with Enter
      await user.keyboard('{Enter}')

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          '/api/fabric/send-message',
          expect.objectContaining({
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: expect.stringContaining('"workflowId":"test-workflow"'),
          })
        )
      })

      // Verify the body contains the correct data
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const sendCall = mockFetch.mock.calls.find((call: any) => call[0] === '/api/fabric/send-message')
      expect(sendCall).toBeDefined()
      if (!sendCall) throw new Error('sendCall not found')
      const body = JSON.parse(sendCall[1].body)
      expect(body.workflowId).toBe('test-workflow')
      expect(body.channelSlug).toBe('tasks')
      // After autocomplete selection, the content should be 'Hello world @coordinator '
      expect(body.content).toBe('Hello world @coordinator')
      expect(body.mentions).toContain('coordinator')
    })

    it('handleThreadReply calls /api/fabric/reply with correct params', async () => {
      const user = userEvent.setup()

      // Add a message with a reply to create a thread
      const eventsWithThread: FabricEvent[] = [
        ...sampleEvents,
        {
          version: 1,
          timestamp: new Date().toISOString(),
          event: {
            type: 'reply.posted',
            timestamp: new Date().toISOString(),
            channel_id: 'ch-tasks',
            parent_id: 'msg-1',
            thread: {
              id: 'reply-1',
              type: 'reply',
              created_at: new Date().toISOString(),
              created_by: 'worker-1',
              content: 'This is a reply',
              seq: 3,
            },
          },
        },
      ]

      render(<FabricPanel events={eventsWithThread} workflowId="test-workflow" />)

      // Wait for agents to load
      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/api/fabric/agents')
        )
      })

      // Click on the reply indicator to open thread panel
      const replyIndicator = screen.getByText('1 reply')
      await user.click(replyIndicator)

      // Now find the thread reply input
      const threadInput = screen.getByPlaceholderText('Reply to thread...')
      await user.type(threadInput, 'My reply')
      await user.keyboard('{Enter}')

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          '/api/fabric/reply',
          expect.objectContaining({
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
          })
        )
      })

      // Verify the body contains the correct data
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const replyCall = mockFetch.mock.calls.find((call: any) => call[0] === '/api/fabric/reply')
      expect(replyCall).toBeDefined()
      if (!replyCall) throw new Error('replyCall not found')
      const body = JSON.parse(replyCall[1].body)
      expect(body.workflowId).toBe('test-workflow')
      expect(body.threadId).toBe('msg-1')
      expect(body.content).toBe('My reply')
    })
  })

  describe('workflowId flow', () => {
    it('passes workflowId to API calls correctly', async () => {
      const user = userEvent.setup()

      render(<FabricPanel events={sampleEvents} workflowId="my-custom-workflow" />)

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('workflowId=my-custom-workflow')
        )
      })

      const input = screen.getByPlaceholderText('Message #Tasks...')
      await user.type(input, 'Test message')
      await user.keyboard('{Enter}')

      await waitFor(() => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const sendCall = mockFetch.mock.calls.find((call: any) => call[0] === '/api/fabric/send-message')
        expect(sendCall).toBeDefined()
        if (!sendCall) throw new Error('sendCall not found')
        const body = JSON.parse(sendCall[1].body)
        expect(body.workflowId).toBe('my-custom-workflow')
      })
    })
  })

  describe('ChatInput placement', () => {
    it('ChatInput appears in channel view below message-list', () => {
      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      // Find the channel input container
      const channelInputContainer = document.querySelector('.channel-input-container')
      expect(channelInputContainer).toBeInTheDocument()

      // The ChatInput inside should have the channel placeholder
      const input = screen.getByPlaceholderText('Message #Tasks...')
      expect(input).toBeInTheDocument()
    })

    it('ChatInput appears in thread panel below thread-content', async () => {
      const user = userEvent.setup()

      // Add a message with a reply to create a thread
      const eventsWithThread: FabricEvent[] = [
        ...sampleEvents,
        {
          version: 1,
          timestamp: new Date().toISOString(),
          event: {
            type: 'reply.posted',
            timestamp: new Date().toISOString(),
            channel_id: 'ch-tasks',
            parent_id: 'msg-1',
            thread: {
              id: 'reply-1',
              type: 'reply',
              created_at: new Date().toISOString(),
              created_by: 'worker-1',
              content: 'This is a reply',
              seq: 3,
            },
          },
        },
      ]

      render(<FabricPanel events={eventsWithThread} workflowId="test-workflow" />)

      // Click on the reply indicator to open thread panel
      const replyIndicator = screen.getByText('1 reply')
      await user.click(replyIndicator)

      // Find the thread input container
      const threadInputContainer = document.querySelector('.thread-input-container')
      expect(threadInputContainer).toBeInTheDocument()

      // The ChatInput inside should have the thread placeholder
      const threadInput = screen.getByPlaceholderText('Reply to thread...')
      expect(threadInput).toBeInTheDocument()
    })
  })

  describe('error handling', () => {
    it('handles agents fetch failure gracefully', async () => {
      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/fabric/agents')) {
          return Promise.reject(new Error('Network error'))
        }
        return Promise.reject(new Error('Unknown endpoint'))
      })

      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      await waitFor(() => {
        // Should disable input when fetch fails
        const input = screen.getByPlaceholderText('Message #Tasks...')
        expect(input).toBeDisabled()
      })
    })

    it('handles send message failure gracefully', async () => {
      const user = userEvent.setup()

      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/fabric/agents')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve(mockAgentsResponse),
          })
        }
        if (url.includes('/api/fabric/send-message')) {
          return Promise.resolve({
            ok: false,
            json: () => Promise.resolve({ error: 'Failed to send' }),
          })
        }
        return Promise.reject(new Error('Unknown endpoint'))
      })

      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/api/fabric/agents')
        )
      })

      const input = screen.getByPlaceholderText('Message #Tasks...')
      await user.type(input, 'Test message')
      await user.keyboard('{Enter}')

      // Input should preserve content on failure (from ChatInput component)
      await waitFor(() => {
        expect(input).toHaveValue('Test message')
      })
    })

    it('shows error toast on API failure', async () => {
      const user = userEvent.setup()

      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/fabric/agents')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve(mockAgentsResponse),
          })
        }
        if (url.includes('/api/fabric/send-message')) {
          return Promise.resolve({
            ok: false,
            json: () => Promise.resolve({ error: 'Server error: rate limited' }),
          })
        }
        return Promise.reject(new Error('Unknown endpoint'))
      })

      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/api/fabric/agents')
        )
      })

      const input = screen.getByPlaceholderText('Message #Tasks...')
      await user.type(input, 'Test message')
      await user.keyboard('{Enter}')

      // Toast should appear with error message
      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument()
        expect(screen.getByText('Server error: rate limited')).toBeInTheDocument()
      })
    })

    it('shows network error toast on fetch failure', async () => {
      const user = userEvent.setup()

      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/fabric/agents')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve(mockAgentsResponse),
          })
        }
        if (url.includes('/api/fabric/send-message')) {
          return Promise.reject(new TypeError('Failed to fetch'))
        }
        return Promise.reject(new Error('Unknown endpoint'))
      })

      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/api/fabric/agents')
        )
      })

      const input = screen.getByPlaceholderText('Message #Tasks...')
      await user.type(input, 'Test message')
      await user.keyboard('{Enter}')

      // Toast should appear with network error message
      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument()
        expect(screen.getByText('Network error. Please try again.')).toBeInTheDocument()
      })
    })

    it('preserves message content when send fails', async () => {
      const user = userEvent.setup()

      mockFetch.mockImplementation((url: string) => {
        if (url.includes('/api/fabric/agents')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve(mockAgentsResponse),
          })
        }
        if (url.includes('/api/fabric/send-message')) {
          return Promise.resolve({
            ok: false,
            json: () => Promise.resolve({ error: 'Send failed' }),
          })
        }
        return Promise.reject(new Error('Unknown endpoint'))
      })

      render(<FabricPanel events={sampleEvents} workflowId="test-workflow" />)

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/api/fabric/agents')
        )
      })

      const input = screen.getByPlaceholderText('Message #Tasks...')
      await user.type(input, 'My important message')
      await user.keyboard('{Enter}')

      // Message content should be preserved for retry
      await waitFor(() => {
        expect(input).toHaveValue('My important message')
      })
    })
  })
})
