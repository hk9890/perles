package handler

// SessionRefNotifier is called when a process's session reference is captured.
// This allows the session service to persist the ref for resumption.
// Implementations must be thread-safe.
type SessionRefNotifier interface {
	// NotifySessionRef is called after a process's first successful turn.
	// processID: "coordinator" or "worker-N"
	// sessionRef: the headless client session ID
	// workDir: the process's working directory
	NotifySessionRef(processID, sessionRef, workDir string) error
}
