package mcp

import "time"

// JoinResponse is the response for fabric_join.
type JoinResponse struct {
	AgentID string `json:"agent_id"`
	Role    string `json:"role"`
	Message string `json:"message"`
}

// InboxResponse is the response for fabric_inbox.
type InboxResponse struct {
	Channels     []ChannelInbox `json:"channels"`
	TotalUnacked int            `json:"total_unacked"`
}

// ChannelInbox contains unread messages for a single channel.
type ChannelInbox struct {
	ChannelID   string         `json:"channel_id"`
	ChannelSlug string         `json:"channel_slug"`
	Unacked     int            `json:"unacked"`
	Messages    []InboxMessage `json:"messages"`
}

// InboxMessage is a message summary in the inbox.
type InboxMessage struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Mentions  []string  `json:"mentions,omitempty"`
}

// SendResponse is the response for fabric_send.
type SendResponse struct {
	ID        string   `json:"id"`
	Seq       int64    `json:"seq"`
	ChannelID string   `json:"channel_id"`
	Mentions  []string `json:"mentions,omitempty"`
}

// ReplyResponse is the response for fabric_reply.
type ReplyResponse struct {
	ID             string   `json:"id"`
	Seq            int64    `json:"seq"`
	ParentID       string   `json:"parent_id"`
	Mentions       []string `json:"mentions,omitempty"`
	ThreadDepth    int      `json:"thread_depth"`
	ThreadPosition int      `json:"thread_position"`
}

// AckResponse is the response for fabric_ack.
type AckResponse struct {
	AckedCount int `json:"acked_count"`
}

// SubscribeResponse is the response for fabric_subscribe.
type SubscribeResponse struct {
	ChannelID string `json:"channel_id"`
	Mode      string `json:"mode"`
}

// UnsubscribeResponse is the response for fabric_unsubscribe.
type UnsubscribeResponse struct {
	Success bool `json:"success"`
}

// AttachResponse is the response for fabric_attach.
type AttachResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}

// HistoryResponse is the response for fabric_history.
type HistoryResponse struct {
	ChannelID   string           `json:"channel_id"`
	ChannelSlug string           `json:"channel_slug"`
	Messages    []HistoryMessage `json:"messages"`
	TotalCount  int              `json:"total_count"`
}

// HistoryMessage is a message in the channel history.
type HistoryMessage struct {
	ID          string    `json:"id"`
	Seq         int64     `json:"seq"`
	Content     string    `json:"content"`
	Kind        string    `json:"kind"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	ReplyCount  int       `json:"reply_count"`
	IsAcked     bool      `json:"is_acked"`
	Mentions    []string  `json:"mentions,omitempty"`
	HasArtifact bool      `json:"has_artifact"`
}

// ReadThreadResponse is the response for fabric_read_thread.
type ReadThreadResponse struct {
	Message      ThreadMessage    `json:"message"`
	Replies      []ThreadMessage  `json:"replies"`
	Artifacts    []ThreadArtifact `json:"artifacts,omitempty"`
	Participants []string         `json:"participants"`
}

// ThreadMessage is a message in a thread.
type ThreadMessage struct {
	ID        string    `json:"id"`
	Seq       int64     `json:"seq"`
	Content   string    `json:"content"`
	Kind      string    `json:"kind"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Mentions  []string  `json:"mentions,omitempty"`
}

// ThreadArtifact is an artifact attached to a thread.
type ThreadArtifact struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MediaType string    `json:"media_type"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Preview   string    `json:"preview,omitempty"`
}
