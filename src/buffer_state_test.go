package src

import (
	"fmt"
	"sync"
	"testing"
)

// TestBufferStateIsRaceFree hammers the shared Playlist maps from every
// direction that touches them in production: clients joining and syncing their
// segment history, the producer flipping the stream status, and the WebUI
// dashboard walking and repairing the client maps.
//
// A Playlist is stored in a sync.Map by value but carries two maps, so every
// copy shares them. Before these accessors were introduced, concurrent access
// here was a "concurrent map read and map write" - a runtime fatal error that
// no recover can catch, which simply kills Threadfin.
func TestBufferStateIsRaceFree(t *testing.T) {
	const playlistID = "M4YKP42H8O5GZ4B236G6"

	playlist := Playlist{
		Folder:       "/tmp/threadfinx-test/state/",
		PlaylistID:   playlistID,
		PlaylistName: "Movistar",
		Buffer:       "ffmpeg",
		Tuner:        4,
		Clients:      make(map[int]ThisClient),
		Streams:      make(map[int]ThisStream),
	}

	registerPlaylist(playlist)
	t.Cleanup(func() { BufferInformation.Delete(playlistID) })

	for i := 0; i < 4; i++ {
		addStream(playlistID, i, ThisStream{
			ChannelName: fmt.Sprintf("ch-%d", i),
			MD5:         fmt.Sprintf("md5-%d", i),
			PlaylistID:  playlistID,
			URL:         fmt.Sprintf("http://192.168.1.211:8080/live/admin/movistar/%d", i),
		})
	}

	var wg sync.WaitGroup

	// Clients joining and leaving.
	for c := 0; c < 4; c++ {
		streamID := c

		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				joinStream(playlistID, streamID)
				getStreamSnapshot(playlistID, streamID)
			}
		}()
	}

	// Clients syncing their segment history back.
	for c := 0; c < 4; c++ {
		streamID := c

		wg.Add(1)
		go func() {
			defer wg.Done()

			var history []string
			for i := 0; i < 200; i++ {
				history = append(history, fmt.Sprintf("%d.ts", i))
				if len(history) > 20 {
					history = history[len(history)-20:]
				}
				syncOldSegments(playlistID, streamID, history)
			}
		}()
	}

	// The producer marking streams as live.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			setStreamStatus(playlistID, i%4, i%2 == 0)
		}
	}()

	// The WebUI dashboard, which both reads and repairs the client maps.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			getActiveClientCount()
			getActivePlaylistCount()
			getStreamCount(playlistID)
		}
	}()

	// Lookups for restreaming.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			findStreamByURL(playlistID, fmt.Sprintf("http://192.168.1.211:8080/live/admin/movistar/%d", i%4))
			nextStreamID(playlistID)
		}
	}()

	wg.Wait()
}

// TestSyncOldSegmentsCopiesTheSlice: handing the caller's own slice to the
// shared map would leave the client and every future reader appending into the
// same backing array.
func TestSyncOldSegmentsCopiesTheSlice(t *testing.T) {
	const playlistID = "M-slice-test"

	registerPlaylist(Playlist{
		PlaylistID: playlistID,
		Clients:    make(map[int]ThisClient),
		Streams:    make(map[int]ThisStream),
	})
	t.Cleanup(func() { BufferInformation.Delete(playlistID) })

	addStream(playlistID, 0, ThisStream{MD5: "md5"})

	history := make([]string, 0, 8)
	history = append(history, "1.ts", "2.ts")

	syncOldSegments(playlistID, 0, history)

	// Reuse the caller's spare capacity, as append in the client loop does.
	history = append(history[:1], "overwritten.ts")

	stored, ok := getStreamSnapshot(playlistID, 0)
	if !ok {
		t.Fatal("stream disappeared")
	}

	if len(stored.OldSegments) != 2 || stored.OldSegments[1] != "2.ts" {
		t.Fatalf("stored history aliases the caller's slice: %v", stored.OldSegments)
	}

	_ = history
}

// TestJoinStreamCountsEveryClient: the connection count decides when the buffer
// is torn down, so a lost increment means tearing a stream down under a viewer.
func TestJoinStreamCountsEveryClient(t *testing.T) {
	const playlistID = "M-join-test"

	registerPlaylist(Playlist{
		PlaylistID: playlistID,
		Clients:    make(map[int]ThisClient),
		Streams:    make(map[int]ThisStream),
	})
	t.Cleanup(func() { BufferInformation.Delete(playlistID) })

	addStream(playlistID, 0, ThisStream{MD5: "md5"})

	const joins = 100

	var wg sync.WaitGroup
	for i := 0; i < joins; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			joinStream(playlistID, 0)
		}()
	}
	wg.Wait()

	Lock.RLock()
	p, _ := BufferInformation.Load(playlistID)
	got := p.(Playlist).Clients[0].Connection
	Lock.RUnlock()

	// addStream registers the first client, then every join adds one.
	if want := 1 + joins; got != want {
		t.Fatalf("connection count = %d, want %d: increments were lost to a race", got, want)
	}
}
