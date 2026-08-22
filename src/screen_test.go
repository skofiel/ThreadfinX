package src

import (
	"fmt"
	"sync"
	"testing"
)

func withLogLimit(t *testing.T, limit int) {
	t.Helper()

	restore := Settings.LogEntriesRAM
	t.Cleanup(func() {
		Settings.LogEntriesRAM = restore
		logMutex.Lock()
		WebScreenLog.Log = nil
		logMutex.Unlock()
	})

	Settings.LogEntriesRAM = limit

	logMutex.Lock()
	WebScreenLog.Log = nil
	logMutex.Unlock()
}

// TestLogCleanUpKeepsTheNewestEntries covers both bugs in the old trim: it
// dropped the newest line in the ordinary case, and once the buffer had grown
// past twice the limit the loop did not run at all and the log was wiped.
func TestLogCleanUpKeepsTheNewestEntries(t *testing.T) {
	const limit = 10

	for _, total := range []int{limit + 1, limit * 2, limit*3 + 7} {
		withLogLimit(t, limit)

		logMutex.Lock()
		for i := 0; i < total; i++ {
			WebScreenLog.Log = append(WebScreenLog.Log, fmt.Sprintf("line-%d", i))
		}
		logMutex.Unlock()

		logCleanUp()

		logMutex.Lock()
		got := append([]string(nil), WebScreenLog.Log...)
		logMutex.Unlock()

		if len(got) != limit {
			t.Fatalf("total=%d: kept %d entries, want %d", total, len(got), limit)
		}

		if want := fmt.Sprintf("line-%d", total-1); got[len(got)-1] != want {
			t.Fatalf("total=%d: newest entry is %q, want %q", total, got[len(got)-1], want)
		}

		if want := fmt.Sprintf("line-%d", total-limit); got[0] != want {
			t.Fatalf("total=%d: oldest kept entry is %q, want %q", total, got[0], want)
		}
	}
}

func TestLogCleanUpCountsWarningsAndErrors(t *testing.T) {
	withLogLimit(t, 100)

	logMutex.Lock()
	WebScreenLog.Log = []string{"ok", "[WARNING] hmm", "[ERROR] bad", "[ERROR] worse"}
	logMutex.Unlock()

	logCleanUp()

	if WebScreenLog.Warnings != 1 {
		t.Errorf("warnings = %d, want 1", WebScreenLog.Warnings)
	}

	if WebScreenLog.Errors != 2 {
		t.Errorf("errors = %d, want 2", WebScreenLog.Errors)
	}
}

// TestWebLogIsRaceFree: every logging call appends to WebScreenLog while the
// WebUI handler reads it. The mutexes these functions used to take were local
// variables, freshly made on each call, so they synchronised nothing.
func TestWebLogIsRaceFree(t *testing.T) {
	withLogLimit(t, 50)

	var wg sync.WaitGroup

	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				appendToWebLog(fmt.Sprintf("[%d] line %d", id, i))
			}
		}(w)
	}

	// The WebUI dashboard copying the log out.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			logMutex.Lock()
			_ = append([]string(nil), WebScreenLog.Log...)
			logMutex.Unlock()
		}
	}()

	wg.Wait()

	logMutex.Lock()
	got := len(WebScreenLog.Log)
	logMutex.Unlock()

	if got > 50 {
		t.Fatalf("log grew past the configured limit: %d entries", got)
	}
}
