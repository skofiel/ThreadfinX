package src

import (
	"fmt"
	"sync"
	"testing"
)

func resetCacheForTest(t *testing.T) {
	t.Helper()
	resetStreamingURLCache()
	t.Cleanup(resetStreamingURLCache)
}

// TestRebuildRetiresRotatedProviderIDs is the provider-rotation case: the
// provider drops a channel id and publishes a new one. After the rebuild the
// retired id must stop resolving, or Threadfin keeps launching FFmpeg against a
// dead URL for as long as Plex remembers the old link.
func TestRebuildRetiresRotatedProviderIDs(t *testing.T) {
	resetCacheForTest(t)

	const playlist = "M4YKP42H8O5GZ4B236G6"

	oldURL, err := createStreamingURL("M3U", playlist, "1", "DAZN 1",
		"http://192.168.1.211:8080/live/admin/movistar/316503", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	oldID := getMD5(fmt.Sprintf("%s-%s", playlist, "http://192.168.1.211:8080/live/admin/movistar/316503"))

	if _, ok := lookupStreamingURL(oldID); !ok {
		t.Fatalf("channel not cached (%s)", oldURL)
	}

	// The provider rotates the id; the next rebuild only publishes the new URL.
	beginStreamingURLRebuild()

	if _, ok := lookupStreamingURL(oldID); !ok {
		t.Fatal("the retired id stopped resolving during the rebuild; " +
			"that is the 404 window that disconnects active players")
	}

	newRawURL := "http://192.168.1.211:8080/live/admin/movistar/999999"
	if _, err := createStreamingURL("M3U", playlist, "1", "DAZN 1", newRawURL, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	newID := getMD5(fmt.Sprintf("%s-%s", playlist, newRawURL))

	_, dropped := commitStreamingURLRebuild()

	if dropped != 1 {
		t.Fatalf("want 1 retired id, got %d", dropped)
	}

	if _, ok := lookupStreamingURL(oldID); ok {
		t.Fatal("the rotated-away id still resolves after the rebuild")
	}

	if _, ok := lookupStreamingURL(newID); !ok {
		t.Fatal("the new id does not resolve after the rebuild")
	}
}

// TestRebuildKeepsChannelsResolvableThroughout guards the regression 05ae5d6
// was written for: no channel may become unresolvable while a rebuild runs.
func TestRebuildKeepsChannelsResolvableThroughout(t *testing.T) {
	resetCacheForTest(t)

	const playlist = "M4YKP42H8O5GZ4B236G6"

	var ids []string
	for i := 0; i < 50; i++ {
		raw := fmt.Sprintf("http://192.168.1.211:8080/live/admin/movistar/%d", i)
		if _, err := createStreamingURL("M3U", playlist, fmt.Sprint(i), "ch", raw, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, getMD5(fmt.Sprintf("%s-%s", playlist, raw)))
	}

	var (
		wg       sync.WaitGroup
		stop     = make(chan struct{})
		failures = make(chan string, 1)
	)

	// A player resolving channels continuously, as Plex does.
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-stop:
				return
			default:
			}

			for _, id := range ids {
				if _, ok := lookupStreamingURL(id); !ok {
					select {
					case failures <- id:
					default:
					}
					return
				}
			}
		}
	}()

	// The rebuild, republishing every channel unchanged.
	beginStreamingURLRebuild()

	for i := 0; i < 50; i++ {
		raw := fmt.Sprintf("http://192.168.1.211:8080/live/admin/movistar/%d", i)
		if _, err := createStreamingURL("M3U", playlist, fmt.Sprint(i), "ch", raw, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	if _, dropped := commitStreamingURLRebuild(); dropped != 0 {
		t.Fatalf("rebuild dropped %d unchanged channels", dropped)
	}

	close(stop)
	wg.Wait()

	select {
	case id := <-failures:
		t.Fatalf("channel %s became unresolvable during the rebuild", id)
	default:
	}
}

// TestFailedRebuildKeepsExistingChannels: a rebuild that errors out must not
// take the working channel set down with it.
func TestFailedRebuildKeepsExistingChannels(t *testing.T) {
	resetCacheForTest(t)

	const playlist = "M4YKP42H8O5GZ4B236G6"
	raw := "http://192.168.1.211:8080/live/admin/movistar/316638"

	if _, err := createStreamingURL("M3U", playlist, "1", "M+ LaLiga", raw, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	id := getMD5(fmt.Sprintf("%s-%s", playlist, raw))

	beginStreamingURLRebuild()
	abortStreamingURLRebuild()

	if _, ok := lookupStreamingURL(id); !ok {
		t.Fatal("an aborted rebuild dropped a channel that is still valid")
	}
}

// TestStreamingURLCacheIsRaceFree exercises the read path against the rebuild
// path. Before the mutex this was a "concurrent map read and map write" fatal
// error, which no recover can catch: it kills the process.
func TestStreamingURLCacheIsRaceFree(t *testing.T) {
	resetCacheForTest(t)

	const playlist = "M4YKP42H8O5GZ4B236G6"

	var wg sync.WaitGroup

	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				lookupStreamingURL(getMD5(fmt.Sprintf("%s-%d", playlist, i)))
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for round := 0; round < 5; round++ {
			beginStreamingURLRebuild()
			for i := 0; i < 100; i++ {
				raw := fmt.Sprintf("http://192.168.1.211:8080/live/admin/movistar/%d", i)
				createStreamingURL("M3U", playlist, fmt.Sprint(i), "ch", raw, nil, nil, nil)
			}
			commitStreamingURLRebuild()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			snapshotStreamingURLs()
		}
	}()

	wg.Wait()
}
