package persistence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zjrosen/perles/internal/orchestration/fabric"
	"github.com/zjrosen/perles/internal/orchestration/fabric/domain"
	"github.com/zjrosen/perles/internal/orchestration/fabric/repository"
)

// LoadPersistedEvents loads all persisted Fabric events from a session directory.
// Returns an empty slice if the file doesn't exist.
// Malformed JSON lines are skipped gracefully to provide resilience against partial writes.
func LoadPersistedEvents(sessionDir string) ([]PersistedEvent, error) {
	filePath := filepath.Join(sessionDir, FabricEventsFile)

	file, err := os.Open(filePath) //nolint:gosec // path is constructed internally
	if err != nil {
		if os.IsNotExist(err) {
			return []PersistedEvent{}, nil
		}
		return nil, fmt.Errorf("opening fabric events file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var events []PersistedEvent
	scanner := bufio.NewScanner(file)

	// Increase buffer size for potentially long lines (artifact content)
	buf := make([]byte, maxLineSize)
	scanner.Buffer(buf, maxLineSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var pe PersistedEvent
		if err := json.Unmarshal(line, &pe); err != nil {
			// Skip malformed lines - provides resilience against partial writes
			continue
		}
		events = append(events, pe)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning fabric events file: %w", err)
	}

	// Ensure we return an empty slice, not nil
	if events == nil {
		events = []PersistedEvent{}
	}

	return events, nil
}

// RestoreFabricState rebuilds Fabric repository state from persisted events.
// This function replays events in order to reconstruct:
// - Threads (channels, messages, artifacts)
// - Dependencies (graph edges)
// - Subscriptions
// - Acks
// - Participants
// - Reactions
//
// Note: This creates new entities directly in repositories without triggering
// new events, which is appropriate for restoration.
func RestoreFabricState(
	events []PersistedEvent,
	threads repository.ThreadRepository,
	deps repository.DependencyRepository,
	subs repository.SubscriptionRepository,
	acks repository.AckRepository,
	participants repository.ParticipantRepository,
	reactions repository.ReactionRepository,
) error {
	for _, pe := range events {
		if err := replayEvent(pe, threads, deps, subs, acks, participants, reactions); err != nil {
			// Log warning but continue - don't fail on one bad event
			// This provides resilience against corrupted events
			continue
		}
	}
	return nil
}

// replayEvent processes a single persisted event and updates repositories accordingly.
func replayEvent(
	pe PersistedEvent,
	threads repository.ThreadRepository,
	deps repository.DependencyRepository,
	subs repository.SubscriptionRepository,
	acks repository.AckRepository,
	participants repository.ParticipantRepository,
	reactions repository.ReactionRepository,
) error {
	event := pe.Event

	switch event.Type {
	case fabric.EventChannelCreated:
		return replayChannelCreated(event, threads, deps)

	case fabric.EventMessagePosted:
		return replayMessagePosted(event, threads, deps)

	case fabric.EventReplyPosted:
		return replayReplyPosted(event, threads, deps)

	case fabric.EventArtifactAdded:
		return replayArtifactAdded(pe, threads, deps)

	case fabric.EventSubscribed:
		return replaySubscribed(event, subs)

	case fabric.EventUnsubscribed:
		return replayUnsubscribed(event, subs)

	case fabric.EventAcked:
		return replayAcked(event, acks)

	case fabric.EventChannelArchived:
		return replayChannelArchived(event, threads)

	case fabric.EventParticipantJoined:
		return replayParticipantJoined(event, participants)

	case fabric.EventParticipantLeft:
		return replayParticipantLeft(event, participants)

	case fabric.EventReactionAdded:
		return replayReactionAdded(event, reactions)

	case fabric.EventReactionRemoved:
		return replayReactionRemoved(event, reactions)

	default:
		// Unknown event type - skip
		return nil
	}
}

// replayChannelCreated restores a channel from its creation event.
func replayChannelCreated(event fabric.Event, threads repository.ThreadRepository, _ repository.DependencyRepository) error {
	if event.Thread == nil {
		return fmt.Errorf("channel created event has no thread")
	}

	// Create the channel thread (preserving the original ID)
	thread := *event.Thread
	if _, err := threads.Create(thread); err != nil {
		// May already exist if we're replaying partial logs
		return nil
	}

	return nil
}

// replayMessagePosted restores a message and its child_of dependency.
func replayMessagePosted(event fabric.Event, threads repository.ThreadRepository, deps repository.DependencyRepository) error {
	if event.Thread == nil {
		return fmt.Errorf("message posted event has no thread")
	}

	// Create the message thread
	thread := *event.Thread
	if _, err := threads.Create(thread); err != nil {
		return nil // May already exist
	}

	// Create child_of dependency to the channel
	if event.ChannelID != "" {
		dep := domain.NewDependency(thread.ID, event.ChannelID, domain.RelationChildOf)
		_ = deps.Add(dep)
	}

	return nil
}

// replayReplyPosted restores a reply message and its reply_to dependency.
func replayReplyPosted(event fabric.Event, threads repository.ThreadRepository, _ repository.DependencyRepository) error {
	if event.Thread == nil {
		return fmt.Errorf("reply posted event has no thread")
	}

	// Create the reply thread
	thread := *event.Thread
	if _, err := threads.Create(thread); err != nil {
		return nil // May already exist
	}

	// Note: The reply_to dependency should be inferred from Thread.Meta["reply_to"]
	// or stored in the event. For now, we rely on the ChannelID field being
	// repurposed for the parent message ID in reply events.
	// This matches the behavior in service.go where Reply uses findChannelForMessage.

	return nil
}

// replayArtifactAdded restores an artifact and its references dependency.
// Note: Artifact content is not stored - the artifact references a file by path (StorageURI).
func replayArtifactAdded(pe PersistedEvent, threads repository.ThreadRepository, deps repository.DependencyRepository) error {
	event := pe.Event
	if event.Thread == nil {
		return fmt.Errorf("artifact added event has no thread")
	}

	// Create the artifact thread (contains StorageURI pointing to file)
	thread := *event.Thread
	if _, err := threads.Create(thread); err != nil {
		return nil // May already exist
	}

	// Create references dependency to the target (channel or message)
	if event.ChannelID != "" {
		dep := domain.NewDependency(thread.ID, event.ChannelID, domain.RelationReferences)
		_ = deps.Add(dep)
	}

	return nil
}

// replaySubscribed restores a subscription.
func replaySubscribed(event fabric.Event, subs repository.SubscriptionRepository) error {
	if event.Subscription == nil {
		return fmt.Errorf("subscribed event has no subscription")
	}

	_, _ = subs.Subscribe(
		event.Subscription.ChannelID,
		event.Subscription.AgentID,
		event.Subscription.Mode,
	)
	return nil
}

// replayUnsubscribed removes a subscription.
func replayUnsubscribed(event fabric.Event, subs repository.SubscriptionRepository) error {
	if event.ChannelID == "" || event.AgentID == "" {
		return fmt.Errorf("unsubscribed event missing channel or agent ID")
	}

	_ = subs.Unsubscribe(event.ChannelID, event.AgentID)
	return nil
}

// replayAcked restores ack records.
func replayAcked(event fabric.Event, acks repository.AckRepository) error {
	if event.AgentID == "" {
		return fmt.Errorf("acked event has no agent ID")
	}

	// Thread IDs are stored in the Mentions field (see NewAckedEvent)
	if len(event.Mentions) > 0 {
		_ = acks.Ack(event.AgentID, event.Mentions...)
	}

	return nil
}

// replayChannelArchived marks a channel as archived.
func replayChannelArchived(event fabric.Event, threads repository.ThreadRepository) error {
	if event.ChannelID == "" {
		return fmt.Errorf("channel archived event has no channel ID")
	}

	_ = threads.Archive(event.ChannelID)
	return nil
}

// replayParticipantJoined restores a participant from a join event.
func replayParticipantJoined(event fabric.Event, participants repository.ParticipantRepository) error {
	if event.Participant == nil {
		return fmt.Errorf("participant joined event has no participant")
	}

	_, _ = participants.Join(event.Participant.AgentID, event.Participant.Role)
	return nil
}

// replayParticipantLeft removes a participant from the registry.
func replayParticipantLeft(event fabric.Event, participants repository.ParticipantRepository) error {
	if event.AgentID == "" {
		return fmt.Errorf("participant left event has no agent ID")
	}

	_ = participants.Leave(event.AgentID)
	return nil
}

// replayReactionAdded restores a reaction from an add event.
func replayReactionAdded(event fabric.Event, reactions repository.ReactionRepository) error {
	if reactions == nil {
		return nil // Reactions not configured
	}
	if event.Reaction == nil {
		return fmt.Errorf("reaction added event has no reaction")
	}

	_, _ = reactions.Add(event.Reaction.ThreadID, event.Reaction.AgentID, event.Reaction.Emoji)
	return nil
}

// replayReactionRemoved removes a reaction from an event.
func replayReactionRemoved(event fabric.Event, reactions repository.ReactionRepository) error {
	if reactions == nil {
		return nil // Reactions not configured
	}
	if event.Reaction == nil {
		return fmt.Errorf("reaction removed event has no reaction")
	}

	_ = reactions.Remove(event.Reaction.ThreadID, event.Reaction.AgentID, event.Reaction.Emoji)
	return nil
}

// RestoreFabricService is a convenience function that loads events from disk
// and restores state into the provided repositories.
// Returns the channel IDs for the fixed channels (root, system, tasks, planning, general).
func RestoreFabricService(
	sessionDir string,
	threads repository.ThreadRepository,
	deps repository.DependencyRepository,
	subs repository.SubscriptionRepository,
	acks repository.AckRepository,
	participants repository.ParticipantRepository,
	reactions repository.ReactionRepository,
) (channelIDs map[string]string, err error) {
	events, err := LoadPersistedEvents(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("loading persisted events: %w", err)
	}

	if err := RestoreFabricState(events, threads, deps, subs, acks, participants, reactions); err != nil {
		return nil, fmt.Errorf("restoring fabric state: %w", err)
	}

	// Extract channel IDs from restored state
	channelIDs = make(map[string]string)
	for _, slug := range []string{domain.SlugRoot, domain.SlugSystem, domain.SlugTasks, domain.SlugPlanning, domain.SlugGeneral} {
		if thread, err := threads.GetBySlug(slug); err == nil {
			channelIDs[slug] = thread.ID
		}
	}

	return channelIDs, nil
}

// HasPersistedFabricState checks if a session directory has Fabric state to restore.
func HasPersistedFabricState(sessionDir string) bool {
	filePath := filepath.Join(sessionDir, FabricEventsFile)
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return info.Size() > 0
}
