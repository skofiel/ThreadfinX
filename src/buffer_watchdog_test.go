package src

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// fakeStreamer writes some data to stdout and then goes silent forever without
// closing the pipe - the behaviour of a provider that accepts the connection
// and stops sending, which is what happens when the VPN exit node is blocked.
// FFmpeg's own reconnect flags make it sit there rather than exiting, so from
// Threadfin's side the process simply never produces another byte and never
// reaches EOF.
func fakeStreamer(t *testing.T, bytesBeforeStall int) string {
	t.Helper()

	var dir = t.TempDir()
	var path = dir + "/fake-ffmpeg"

	var script = fmt.Sprintf(`#!/bin/sh
# Emit some payload, then stall with the pipe still open.
head -c %d /dev/zero
exec sleep 3600
`, bytesBeforeStall)

	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	return path
}

// TestWatchdogEndsStalledStream is the production failure end to end: the
// backend cuts mid-transmission, the transcoder hangs instead of exiting, and a
// client is waiting on data that will never arrive.
//
// Before this change the only timeout armed while tmpSegment == 1, so once the
// first segment had been written reader.Read blocked forever: no data, no EOF,
// the process alive and the tuner occupied until Threadfin was restarted. With
// tuner = 1 that took the whole playlist down.
func TestWatchdogEndsStalledStream(t *testing.T) {
	const (
		playlistID = "M4YKP42H8O5GZ4B236G6"
		streamID   = 0
		folder     = "/tmp/threadfinx-test/watchdog/"
		md5        = "43d35d240139f75d6548e232ff322295"
	)

	initBufferVFS()

	if err := bufferVFS.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}

	restore := Settings
	t.Cleanup(func() { Settings = restore })

	Settings.BufferSize = 64             // KB, so a segment completes quickly
	Settings.BufferInactivityTimeout = 2 // seconds
	Settings.FFmpegPath = fakeStreamer(t, 200*1024)
	Settings.FFmpegOptions = "-i [URL] pipe:1"
	Settings.UserAgent = ""

	stream := ThisStream{
		ChannelName: "DAZN 1",
		Folder:      folder,
		MD5:         md5,
		PlaylistID:  playlistID,
		URL:         "http://192.168.1.211:8080/live/admin/movistar/316503",
	}

	playlist := Playlist{
		Folder:       folder,
		PlaylistID:   playlistID,
		PlaylistName: "Movistar",
		Buffer:       "ffmpeg",
		Tuner:        1,
		Clients:      map[int]ThisClient{streamID: {Connection: 1}},
		Streams:      map[int]ThisStream{streamID: stream},
	}

	BufferInformation.Store(playlistID, playlist)
	t.Cleanup(func() { BufferInformation.Delete(playlistID) })

	// A client is connected and watching.
	BufferClients.Store(playlistID+md5, ClientConnection{Connection: 1})
	t.Cleanup(func() { BufferClients.Delete(playlistID + md5) })

	done := make(chan struct{})
	go func() {
		defer close(done)
		thirdPartyBuffer(streamID, playlistID, false, 0)
	}()

	// Generous margin over the 2s window: the producer may still retry once
	// before giving up, and the fake process has to be reaped each time.
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("thirdPartyBuffer never returned: the stream is hung, which is the bug this guards")
	}

	// The client must have been handed the error rather than left waiting.
	c, ok := BufferClients.Load(playlistID + md5)
	if !ok {
		t.Fatal("the client registration disappeared; the producer must not remove clients")
	}

	clients, ok := c.(ClientConnection)
	if !ok {
		t.Fatalf("unexpected BufferClients value %T", c)
	}

	if clients.Error == nil {
		t.Fatal("the stalled stream ended without telling the client; it would hang until the player gave up")
	}

	if clients.Connection != 1 {
		t.Fatalf("the producer altered the client count (%d); only the client goroutine may do that",
			clients.Connection)
	}

	t.Logf("client was told: %v", clients.Error)
}

// TestWatchdogLeavesHealthyStreamAlone: data keeps arriving, so the watchdog
// must not fire. A false positive here would cut healthy channels every few
// seconds.
func TestWatchdogLeavesHealthyStreamAlone(t *testing.T) {
	restore := Settings
	t.Cleanup(func() { Settings = restore })

	Settings.BufferInactivityTimeout = 2

	var window = inactivityTimeout()
	if window != 2*time.Second {
		t.Fatalf("inactivityTimeout() = %s, want 2s", window)
	}

	// A stream that keeps feeding never lets the idle gap reach the window.
	var last = time.Now()
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		last = time.Now()

		if time.Since(last) >= window {
			t.Fatal("watchdog would fire on a stream that is still delivering data")
		}
	}

	Settings.BufferInactivityTimeout = 0
	if inactivityTimeout() != 0 {
		t.Fatal("buffer.inactivity.timeout = 0 must disable the watchdog")
	}
}
