// Package barrier provides synchronization primitives for VTC tests
package barrier

import (
	"fmt"
	"sync"
	"time"

	"github.com/perbu/GTest/pkg/logging"
)

// Barrier represents a named synchronization barrier
type Barrier struct {
	Name      string
	Count     int           // Number of participants required
	Timeout   time.Duration // Timeout for wait operations
	Cyclic    bool          // If true, barrier resets automatically
	Logger    *logging.Logger

	mutex     sync.Mutex
	current   int           // Current number of waiting participants
	cycle     int           // Current barrier cycle (increments on each sync)
	cond      *sync.Cond    // Condition variable for waiting
}

// New creates a new barrier
func New(name string, logger *logging.Logger) *Barrier {
	b := &Barrier{
		Name:    name,
		Count:   1,
		Timeout: 30 * time.Second, // Default timeout
		Logger:  logger,
		current: 0,
		cycle:   0,
	}
	b.cond = sync.NewCond(&b.mutex)
	return b
}

// Start initializes the barrier with a participant count
func (b *Barrier) Start(count int) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if count < 1 {
		return fmt.Errorf("barrier count must be at least 1")
	}

	b.Count = count
	b.current = 0
	b.cycle = 0
	b.Logger.Debug("Barrier %s started with count %d", b.Name, count)
	return nil
}

// Wait waits for other participants to reach the barrier
func (b *Barrier) Wait() error {
	return b.WaitTimeout(b.Timeout)
}

// WaitTimeout waits with a specific timeout
func (b *Barrier) WaitTimeout(timeout time.Duration) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	entryCycle := b.cycle
	b.current++
	b.Logger.Debug("Barrier %s: %d/%d participants waiting (cycle %d)",
		b.Name, b.current, b.Count, entryCycle)

	if b.current >= b.Count {
		// Last one to arrive - release everyone
		b.Logger.Debug("Barrier %s: releasing all participants (cycle %d -> %d)",
			b.Name, entryCycle, entryCycle+1)
		b.current = 0
		b.cycle++
		b.cond.Broadcast()
		return nil
	}

	// Wait for the barrier to be released (cycle to advance)
	// Set up timeout timer
	timeoutChan := make(chan struct{})
	timer := time.AfterFunc(timeout, func() {
		close(timeoutChan)
		// Wake up all waiters to check their timeout status
		b.cond.Broadcast()
	})
	defer timer.Stop()

	// Wait for either the cycle to advance or timeout
	for b.cycle == entryCycle {
		// Check if we timed out before waiting
		select {
		case <-timeoutChan:
			b.current-- // Remove ourselves from the count
			return fmt.Errorf("barrier %s: timeout after %v (cycle %d)", b.Name, timeout, entryCycle)
		default:
		}

		// Wait on the condition variable
		// This releases the lock and reacquires it when signaled
		b.cond.Wait()
	}

	// We've been released - verify cycle advanced
	b.Logger.Debug("Barrier %s: participant released (cycle %d -> %d)",
		b.Name, entryCycle, b.cycle)
	return nil
}

// Sync is equivalent to Wait - synchronizes at the barrier
func (b *Barrier) Sync() error {
	return b.Wait()
}

// SetTimeout sets the default timeout for this barrier
func (b *Barrier) SetTimeout(timeout time.Duration) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.Timeout = timeout
}

// Reset resets the barrier to its initial state
func (b *Barrier) Reset() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.current = 0
	b.cycle++
	b.cond.Broadcast()
}
