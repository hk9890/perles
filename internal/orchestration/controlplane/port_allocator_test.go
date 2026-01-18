package controlplane

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestNewPortAllocator_DefaultConfig(t *testing.T) {
	alloc := NewPortAllocator(nil)
	require.NotNil(t, alloc)

	// Verify default range by checking port availability
	pa := alloc.(*poolPortAllocator)
	require.Equal(t, DefaultPortRangeStart, pa.startPort)
	require.Equal(t, DefaultPortRangeEnd, pa.endPort)
}

func TestNewPortAllocator_CustomConfig(t *testing.T) {
	config := &PortAllocatorConfig{
		StartPort: 8000,
		EndPort:   8010,
	}
	alloc := NewPortAllocator(config)
	require.NotNil(t, alloc)

	pa := alloc.(*poolPortAllocator)
	require.Equal(t, 8000, pa.startPort)
	require.Equal(t, 8010, pa.endPort)
}

func TestReserve_ReturnsPortInConfiguredRange(t *testing.T) {
	config := &PortAllocatorConfig{
		StartPort: 9000,
		EndPort:   9010,
	}
	alloc := NewPortAllocator(config)
	ctx := context.Background()

	id := NewWorkflowID()
	port, release, err := alloc.Reserve(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, release)
	require.GreaterOrEqual(t, port, 9000)
	require.LessOrEqual(t, port, 9010)
}

func TestReserve_ReturnsDifferentPortsForDifferentWorkflows(t *testing.T) {
	alloc := NewPortAllocator(nil)
	ctx := context.Background()

	id1 := NewWorkflowID()
	id2 := NewWorkflowID()
	id3 := NewWorkflowID()

	port1, release1, err := alloc.Reserve(ctx, id1)
	require.NoError(t, err)
	defer release1()

	port2, release2, err := alloc.Reserve(ctx, id2)
	require.NoError(t, err)
	defer release2()

	port3, release3, err := alloc.Reserve(ctx, id3)
	require.NoError(t, err)
	defer release3()

	// All ports should be different
	require.NotEqual(t, port1, port2)
	require.NotEqual(t, port1, port3)
	require.NotEqual(t, port2, port3)
}

func TestReserve_SameWorkflowGetsSamePort(t *testing.T) {
	alloc := NewPortAllocator(nil)
	ctx := context.Background()

	id := NewWorkflowID()

	port1, release1, err := alloc.Reserve(ctx, id)
	require.NoError(t, err)
	defer release1()

	// Same workflow should get same port
	port2, _, err := alloc.Reserve(ctx, id)
	require.NoError(t, err)
	require.Equal(t, port1, port2)
}

func TestRelease_MakesPortAvailableAgain(t *testing.T) {
	config := &PortAllocatorConfig{
		StartPort: 9000,
		EndPort:   9000, // Only one port available
	}
	alloc := NewPortAllocator(config)
	ctx := context.Background()

	id1 := NewWorkflowID()
	id2 := NewWorkflowID()

	// Allocate the only port
	port1, release1, err := alloc.Reserve(ctx, id1)
	require.NoError(t, err)
	require.Equal(t, 9000, port1)

	// Pool should be exhausted
	_, _, err = alloc.Reserve(ctx, id2)
	require.ErrorIs(t, err, ErrNoPortsAvailable)

	// Release the port
	release1()

	// Now id2 should be able to get a port
	port2, release2, err := alloc.Reserve(ctx, id2)
	require.NoError(t, err)
	require.Equal(t, 9000, port2)
	defer release2()
}

func TestReserve_ErrorsWhenPoolExhausted(t *testing.T) {
	config := &PortAllocatorConfig{
		StartPort: 9000,
		EndPort:   9002, // Only 3 ports available
	}
	alloc := NewPortAllocator(config)
	ctx := context.Background()

	// Allocate all ports
	var releases []func()
	for i := 0; i < 3; i++ {
		id := NewWorkflowID()
		_, release, err := alloc.Reserve(ctx, id)
		require.NoError(t, err)
		releases = append(releases, release)
	}

	// Next reservation should fail
	id := NewWorkflowID()
	_, _, err := alloc.Reserve(ctx, id)
	require.ErrorIs(t, err, ErrNoPortsAvailable)

	// Cleanup
	for _, release := range releases {
		release()
	}
}

func TestReserve_RespectsContextCancellation(t *testing.T) {
	alloc := NewPortAllocator(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	id := NewWorkflowID()
	_, _, err := alloc.Reserve(ctx, id)
	require.ErrorIs(t, err, context.Canceled)
}

func TestReleaseAll_FreesAllPortsForWorkflow(t *testing.T) {
	config := &PortAllocatorConfig{
		StartPort: 9000,
		EndPort:   9000, // Only one port available
	}
	alloc := NewPortAllocator(config)
	ctx := context.Background()

	id1 := NewWorkflowID()
	id2 := NewWorkflowID()

	// Allocate to id1
	port1, _, err := alloc.Reserve(ctx, id1)
	require.NoError(t, err)
	require.Equal(t, 9000, port1)

	// Verify port is not available
	require.False(t, alloc.IsPortAvailable(9000))

	// Release all for id1
	alloc.ReleaseAll(id1)

	// Port should now be available
	require.True(t, alloc.IsPortAvailable(9000))

	// id2 should be able to get the port
	port2, release2, err := alloc.Reserve(ctx, id2)
	require.NoError(t, err)
	require.Equal(t, 9000, port2)
	defer release2()
}

func TestReleaseAll_NoopForUnknownWorkflow(t *testing.T) {
	alloc := NewPortAllocator(nil)
	ctx := context.Background()

	id1 := NewWorkflowID()
	id2 := NewWorkflowID()

	// Allocate to id1
	_, release, err := alloc.Reserve(ctx, id1)
	require.NoError(t, err)
	defer release()

	// ReleaseAll for unknown id2 should be a no-op
	alloc.ReleaseAll(id2)

	// id1's port should still be allocated
	allocated := alloc.AllocatedPorts()
	require.Contains(t, allocated, id1)
}

func TestIsPortAvailable_TrueForAvailablePort(t *testing.T) {
	config := &PortAllocatorConfig{
		StartPort: 9000,
		EndPort:   9010,
	}
	alloc := NewPortAllocator(config)

	// All ports in range should be available initially
	for port := 9000; port <= 9010; port++ {
		require.True(t, alloc.IsPortAvailable(port), "port %d should be available", port)
	}
}

func TestIsPortAvailable_FalseForAllocatedPort(t *testing.T) {
	alloc := NewPortAllocator(nil)
	ctx := context.Background()

	id := NewWorkflowID()
	port, release, err := alloc.Reserve(ctx, id)
	require.NoError(t, err)
	defer release()

	require.False(t, alloc.IsPortAvailable(port))
}

func TestIsPortAvailable_FalseForOutOfRangePort(t *testing.T) {
	config := &PortAllocatorConfig{
		StartPort: 9000,
		EndPort:   9010,
	}
	alloc := NewPortAllocator(config)

	// Ports outside range should not be available
	require.False(t, alloc.IsPortAvailable(8999))
	require.False(t, alloc.IsPortAvailable(9011))
}

func TestAllocatedPorts_ReturnsCorrectMapping(t *testing.T) {
	alloc := NewPortAllocator(nil)
	ctx := context.Background()

	id1 := NewWorkflowID()
	id2 := NewWorkflowID()

	port1, release1, err := alloc.Reserve(ctx, id1)
	require.NoError(t, err)
	defer release1()

	port2, release2, err := alloc.Reserve(ctx, id2)
	require.NoError(t, err)
	defer release2()

	allocated := alloc.AllocatedPorts()
	require.Len(t, allocated, 2)
	require.Equal(t, port1, allocated[id1])
	require.Equal(t, port2, allocated[id2])
}

func TestAllocatedPorts_ReturnsCopy(t *testing.T) {
	alloc := NewPortAllocator(nil)
	ctx := context.Background()

	id := NewWorkflowID()
	_, release, err := alloc.Reserve(ctx, id)
	require.NoError(t, err)
	defer release()

	// Get allocated ports and modify the returned map
	allocated := alloc.AllocatedPorts()
	delete(allocated, id)

	// Original allocation should still be intact
	allocated2 := alloc.AllocatedPorts()
	require.Contains(t, allocated2, id)
}

func TestConcurrentReservations_DontAllocateSamePort(t *testing.T) {
	config := &PortAllocatorConfig{
		StartPort: 9000,
		EndPort:   9099, // 100 ports
	}
	alloc := NewPortAllocator(config)
	ctx := context.Background()

	const numGoroutines = 50
	var wg sync.WaitGroup
	ports := make(chan int, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := NewWorkflowID()
			port, _, err := alloc.Reserve(ctx, id)
			if err != nil {
				errors <- err
				return
			}
			ports <- port
		}()
	}

	wg.Wait()
	close(ports)
	close(errors)

	// Check for errors
	for err := range errors {
		require.NoError(t, err)
	}

	// Verify all ports are unique
	seen := make(map[int]bool)
	for port := range ports {
		require.False(t, seen[port], "port %d allocated multiple times", port)
		seen[port] = true
	}

	require.Len(t, seen, numGoroutines)
}

// Property-based tests using rapid

func TestPropertyNoTwoWorkflowsSharePort(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		config := &PortAllocatorConfig{
			StartPort: 9000,
			EndPort:   9010, // Small pool to force collisions if bug exists
		}
		alloc := NewPortAllocator(config)
		ctx := context.Background()

		// Generate random number of workflows (1-10)
		numWorkflows := rapid.IntRange(1, 10).Draw(t, "numWorkflows")

		allocated := make(map[WorkflowID]int)
		var releases []func()

		for i := 0; i < numWorkflows; i++ {
			id := NewWorkflowID()
			port, release, err := alloc.Reserve(ctx, id)
			if err == ErrNoPortsAvailable {
				// Pool exhausted is acceptable
				continue
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			allocated[id] = port
			releases = append(releases, release)
		}

		// Invariant: No two workflows share the same port
		portToWorkflow := make(map[int]WorkflowID)
		for wfID, port := range allocated {
			if existingWF, exists := portToWorkflow[port]; exists {
				t.Fatalf("port %d allocated to both %s and %s", port, existingWF, wfID)
			}
			portToWorkflow[port] = wfID
		}

		// Cleanup
		for _, release := range releases {
			release()
		}
	})
}

func TestPropertyReserveReleaseInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		config := &PortAllocatorConfig{
			StartPort: 9000,
			EndPort:   9005, // Small pool
		}
		alloc := NewPortAllocator(config)
		ctx := context.Background()

		poolSize := 6 // 9000-9005 inclusive

		// Track allocated and released workflows
		type allocation struct {
			id      WorkflowID
			port    int
			release func()
		}
		var allocations []allocation

		// Generate a random sequence of operations
		numOps := rapid.IntRange(10, 50).Draw(t, "numOps")

		for i := 0; i < numOps; i++ {
			// Randomly choose: allocate (0) or release (1)
			if len(allocations) == 0 || rapid.IntRange(0, 1).Draw(t, "op") == 0 {
				// Try to allocate
				id := NewWorkflowID()
				port, release, err := alloc.Reserve(ctx, id)
				if err == ErrNoPortsAvailable {
					// Invariant: pool should be exhausted
					if len(allocations) < poolSize {
						t.Fatalf("got ErrNoPortsAvailable with only %d allocations (pool size %d)",
							len(allocations), poolSize)
					}
					continue
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				allocations = append(allocations, allocation{id: id, port: port, release: release})
			} else {
				// Release a random allocation
				idx := rapid.IntRange(0, len(allocations)-1).Draw(t, "releaseIdx")
				a := allocations[idx]
				a.release()
				// Remove from tracking
				allocations = append(allocations[:idx], allocations[idx+1:]...)
			}
		}

		// Invariant: Number of allocations matches AllocatedPorts count
		allocated := alloc.AllocatedPorts()
		if len(allocated) != len(allocations) {
			t.Fatalf("AllocatedPorts() has %d entries, but tracking has %d",
				len(allocated), len(allocations))
		}

		// Cleanup
		for _, a := range allocations {
			a.release()
		}
	})
}
