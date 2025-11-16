package barrier

import (
	"sync"
	"testing"
	"time"

	"github.com/perbu/gvtest/pkg/logging"
)

func TestBarrier_Basic(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	err := b.Start(1)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = b.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
}

func TestBarrier_TwoParticipants(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	err := b.Start(2)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// First participant
	go func() {
		defer wg.Done()
		err := b.Wait()
		if err != nil {
			t.Errorf("Wait failed for participant 1: %v", err)
		}
	}()

	// Second participant
	go func() {
		defer wg.Done()
		// Small delay to ensure first participant waits
		time.Sleep(10 * time.Millisecond)
		err := b.Wait()
		if err != nil {
			t.Errorf("Wait failed for participant 2: %v", err)
		}
	}()

	wg.Wait()
}

func TestBarrier_MultipleParticipants(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	count := 5
	err := b.Start(count)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func(id int) {
			defer wg.Done()
			err := b.Wait()
			if err != nil {
				t.Errorf("Wait failed for participant %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestBarrier_Timeout(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	err := b.Start(2) // Expect 2 participants
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Only one participant waits, should timeout
	err = b.WaitTimeout(100 * time.Millisecond)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestBarrier_Sync(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	err := b.Start(2)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := b.Sync()
		if err != nil {
			t.Errorf("Sync failed: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		err := b.Sync()
		if err != nil {
			t.Errorf("Sync failed: %v", err)
		}
	}()

	wg.Wait()
}

func TestBarrier_SetTimeout(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	b.SetTimeout(200 * time.Millisecond)
	if b.Timeout != 200*time.Millisecond {
		t.Errorf("SetTimeout failed: expected 200ms, got %v", b.Timeout)
	}
}

func TestBarrier_Reset(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	err := b.Start(2)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	b.Reset()

	// After reset, should be able to use again
	err = b.Start(1)
	if err != nil {
		t.Fatalf("Start after reset failed: %v", err)
	}

	err = b.Wait()
	if err != nil {
		t.Fatalf("Wait after reset failed: %v", err)
	}
}

func TestBarrier_InvalidCount(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	err := b.Start(0)
	if err == nil {
		t.Fatal("Expected error for count=0, got nil")
	}

	err = b.Start(-1)
	if err == nil {
		t.Fatal("Expected error for count=-1, got nil")
	}
}

func TestBarrier_MultipleCycles(t *testing.T) {
	logger := logging.NewLogger("test")
	b := New("b1", logger)

	count := 3
	err := b.Start(count)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Run 3 cycles
	for cycle := 0; cycle < 3; cycle++ {
		var wg sync.WaitGroup
		wg.Add(count)

		for i := 0; i < count; i++ {
			go func() {
				defer wg.Done()
				err := b.Wait()
				if err != nil {
					t.Errorf("Cycle %d: Wait failed: %v", cycle, err)
				}
			}()
		}

		wg.Wait()
	}
}

// Benchmark tests
func BenchmarkBarrier_Wait(b *testing.B) {
	logger := logging.NewLogger("bench")
	barrier := New("b1", logger)
	barrier.Start(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		barrier.Wait()
	}
}

func BenchmarkBarrier_TwoParticipants(b *testing.B) {
	logger := logging.NewLogger("bench")
	barrier := New("b1", logger)
	barrier.Start(2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			barrier.Wait()
		}()

		go func() {
			defer wg.Done()
			barrier.Wait()
		}()

		wg.Wait()
	}
}
