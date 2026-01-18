// Package controlplane provides PortAllocator for managing a pool of ports
// for MCP servers across multiple concurrent workflows.
package controlplane

import (
	"context"
	"fmt"
	"maps"
	"sync"
)

// ErrNoPortsAvailable is returned when the port pool is exhausted.
var ErrNoPortsAvailable = fmt.Errorf("no ports available in pool")

// DefaultPortRangeStart is the default starting port for the port pool.
const DefaultPortRangeStart = 9000

// DefaultPortRangeEnd is the default ending port for the port pool (inclusive).
const DefaultPortRangeEnd = 9100

// PortAllocator manages a pool of ports for MCP servers.
// It provides thread-safe allocation and release of ports,
// tracking which ports are allocated to which workflows.
type PortAllocator interface {
	// Reserve allocates a port for the given workflow ID.
	// Returns the allocated port and a release function, or an error if
	// the pool is exhausted. The release function should be called when
	// the port is no longer needed.
	// The context can be used to cancel a pending reservation.
	Reserve(ctx context.Context, id WorkflowID) (port int, release func(), err error)

	// IsPortAvailable returns true if the given port is available in the pool.
	IsPortAvailable(port int) bool

	// AllocatedPorts returns a map of workflow IDs to their allocated ports.
	// The returned map is a copy and can be safely modified.
	AllocatedPorts() map[WorkflowID]int

	// ReleaseAll releases all ports allocated to the given workflow ID.
	ReleaseAll(id WorkflowID)
}

// PortAllocatorConfig configures the port allocator.
type PortAllocatorConfig struct {
	// StartPort is the first port in the range (inclusive).
	StartPort int
	// EndPort is the last port in the range (inclusive).
	EndPort int
}

// DefaultPortAllocatorConfig returns the default configuration.
func DefaultPortAllocatorConfig() PortAllocatorConfig {
	return PortAllocatorConfig{
		StartPort: DefaultPortRangeStart,
		EndPort:   DefaultPortRangeEnd,
	}
}

// poolPortAllocator is an in-memory implementation of PortAllocator
// that manages a fixed pool of ports.
type poolPortAllocator struct {
	mu sync.Mutex

	// startPort is the first port in the range (inclusive)
	startPort int
	// endPort is the last port in the range (inclusive)
	endPort int

	// allocated maps workflow ID to allocated port
	allocated map[WorkflowID]int
	// used tracks which ports are currently in use (port -> workflow ID)
	used map[int]WorkflowID
}

// NewPortAllocator creates a new PortAllocator with the given configuration.
// If config is nil, the default configuration is used.
func NewPortAllocator(config *PortAllocatorConfig) PortAllocator {
	cfg := DefaultPortAllocatorConfig()
	if config != nil {
		cfg = *config
	}

	return &poolPortAllocator{
		startPort: cfg.StartPort,
		endPort:   cfg.EndPort,
		allocated: make(map[WorkflowID]int),
		used:      make(map[int]WorkflowID),
	}
}

// Reserve allocates a port for the given workflow ID.
func (p *poolPortAllocator) Reserve(ctx context.Context, id WorkflowID) (int, func(), error) {
	// Check context before acquiring lock
	select {
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if workflow already has a port allocated
	if port, ok := p.allocated[id]; ok {
		// Return the existing allocation with a no-op release
		// (the original release function should be used)
		return port, func() {}, nil
	}

	// Find an available port
	for port := p.startPort; port <= p.endPort; port++ {
		if _, used := p.used[port]; !used {
			// Allocate the port
			p.allocated[id] = port
			p.used[port] = id

			// Create release function
			release := func() {
				p.releasePort(id, port)
			}

			return port, release, nil
		}
	}

	return 0, nil, ErrNoPortsAvailable
}

// releasePort releases a specific port for a workflow.
func (p *poolPortAllocator) releasePort(id WorkflowID, port int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Verify the port is still allocated to this workflow
	if allocatedPort, ok := p.allocated[id]; ok && allocatedPort == port {
		delete(p.allocated, id)
		delete(p.used, port)
	}
}

// IsPortAvailable returns true if the given port is available in the pool.
func (p *poolPortAllocator) IsPortAvailable(port int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if port is within range
	if port < p.startPort || port > p.endPort {
		return false
	}

	// Check if port is in use
	_, used := p.used[port]
	return !used
}

// AllocatedPorts returns a map of workflow IDs to their allocated ports.
func (p *poolPortAllocator) AllocatedPorts() map[WorkflowID]int {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Return a copy to avoid external mutation
	result := make(map[WorkflowID]int, len(p.allocated))
	maps.Copy(result, p.allocated)
	return result
}

// ReleaseAll releases all ports allocated to the given workflow ID.
func (p *poolPortAllocator) ReleaseAll(id WorkflowID) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if port, ok := p.allocated[id]; ok {
		delete(p.allocated, id)
		delete(p.used, port)
	}
}
