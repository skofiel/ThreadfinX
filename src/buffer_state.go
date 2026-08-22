package src

/*
Shared buffer state.

BufferInformation is a sync.Map, which makes Load and Store individually safe
and creates the illusion that the data behind them is too. It is not: a Playlist
is stored by value but carries two maps, Clients and Streams, and every copy
handed out by Load shares those same maps. Two goroutines mutating them is a
"concurrent map read and map write", which is a runtime fatal error - it takes
the process down and no recover can stop it.

The unsynchronised writers were:

  - each client goroutine syncing OldSegments back every few segments,
  - the producer flipping stream.Status when the first segment lands,
  - cleanUpStaleClients deleting entries, reached from the WebUI on every
    dashboard refresh,
  - the read-modify-store of a Playlist as a client joins.

Upstream held Lock around these; ebf6103 removed it. Rather than putting the
lock back at the call sites - where it is easy to still be holding it when
calling killClientConnection or clientConnection, which take it themselves and
would deadlock - every access lives here, behind a function that acquires and
releases before returning. Nothing in this file may call a function that takes
Lock.
*/

// getPlaylistSnapshot returns a copy of a playlist's scalar fields. The maps in
// the returned value are the shared ones and must not be touched by the caller;
// use the accessors below instead.
func getPlaylistSnapshot(playlistID string) (playlist Playlist, ok bool) {
	Lock.RLock()
	defer Lock.RUnlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return Playlist{}, false
	}

	playlist, ok = p.(Playlist)

	return playlist, ok
}

// getStreamSnapshot copies one stream out of a playlist.
func getStreamSnapshot(playlistID string, streamID int) (stream ThisStream, ok bool) {
	Lock.RLock()
	defer Lock.RUnlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return ThisStream{}, false
	}

	playlist, valid := p.(Playlist)
	if !valid {
		return ThisStream{}, false
	}

	stream, ok = playlist.Streams[streamID]

	return stream, ok
}

// getStreamCount reports how many streams a playlist is carrying, for the tuner
// limit and the status lines.
func getStreamCount(playlistID string) int {
	Lock.RLock()
	defer Lock.RUnlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return 0
	}

	playlist, ok := p.(Playlist)
	if !ok {
		return 0
	}

	return len(playlist.Streams)
}

// putStream writes a stream back into its playlist.
func putStream(playlistID string, streamID int, stream ThisStream) {
	Lock.Lock()
	defer Lock.Unlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return
	}

	playlist, ok := p.(Playlist)
	if !ok {
		return
	}

	playlist.Streams[streamID] = stream
	BufferInformation.Store(playlistID, playlist)
}

// setStreamStatus marks a stream as producing data. Called by the producer the
// moment the first segment is complete.
func setStreamStatus(playlistID string, streamID int, status bool) {
	Lock.Lock()
	defer Lock.Unlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return
	}

	playlist, ok := p.(Playlist)
	if !ok {
		return
	}

	stream, exists := playlist.Streams[streamID]
	if !exists {
		return
	}

	stream.Status = status
	playlist.Streams[streamID] = stream
	BufferInformation.Store(playlistID, playlist)
}

// syncOldSegments records a client's view of which segments it has already
// sent, so a client joining an existing stream does not replay them.
//
// The slice is copied: handing the client's own slice to the shared map would
// leave both sides appending to the same backing array.
func syncOldSegments(playlistID string, streamID int, oldSegments []string) {
	Lock.Lock()
	defer Lock.Unlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return
	}

	playlist, ok := p.(Playlist)
	if !ok {
		return
	}

	stream, exists := playlist.Streams[streamID]
	if !exists {
		return
	}

	stream.OldSegments = append([]string(nil), oldSegments...)
	playlist.Streams[streamID] = stream
	BufferInformation.Store(playlistID, playlist)
}

// registerPlaylist publishes a freshly built playlist for what the caller
// believes is the first stream on it, and returns the stream id actually in
// effect.
//
// Two clients opening the first channel of a playlist at the same moment both
// find nothing registered and both build a playlist. A plain Store would let
// the second overwrite the first, discarding a stream that already has a
// producer goroutine feeding it. So a playlist that appeared in the meantime is
// merged into instead, under a free id.
func registerPlaylist(playlist Playlist, streamID int) (effectiveID int) {
	Lock.Lock()
	defer Lock.Unlock()

	p, found := BufferInformation.Load(playlist.PlaylistID)
	if !found {
		BufferInformation.Store(playlist.PlaylistID, playlist)
		return streamID
	}

	existing, ok := p.(Playlist)
	if !ok {
		BufferInformation.Store(playlist.PlaylistID, playlist)
		return streamID
	}

	effectiveID = streamID
	for {
		if _, taken := existing.Streams[effectiveID]; !taken {
			break
		}
		effectiveID++
	}

	existing.Streams[effectiveID] = playlist.Streams[streamID]
	existing.Clients[effectiveID] = playlist.Clients[streamID]

	BufferInformation.Store(playlist.PlaylistID, existing)

	return effectiveID
}

// joinStream attaches another client to a stream that is already running and
// returns the resulting connection count. ok is false when the stream is gone.
func joinStream(playlistID string, streamID int) (connections int, ok bool) {
	Lock.Lock()
	defer Lock.Unlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return 0, false
	}

	playlist, valid := p.(Playlist)
	if !valid {
		return 0, false
	}

	if _, exists := playlist.Streams[streamID]; !exists {
		return 0, false
	}

	client := playlist.Clients[streamID]
	client.Connection++
	playlist.Clients[streamID] = client

	BufferInformation.Store(playlistID, playlist)

	return client.Connection, true
}

// addStream adds a new stream (and its first client) to an existing playlist.
func addStream(playlistID string, streamID int, stream ThisStream) {
	Lock.Lock()
	defer Lock.Unlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return
	}

	playlist, ok := p.(Playlist)
	if !ok {
		return
	}

	playlist.Streams[streamID] = stream
	playlist.Clients[streamID] = ThisClient{Connection: 1}

	BufferInformation.Store(playlistID, playlist)
}

// findStreamByURL looks for a stream already serving a URL, so a second viewer
// of the same channel is restreamed rather than opening a second tuner.
func findStreamByURL(playlistID, streamingURL string) (streamID int, stream ThisStream, ok bool) {
	Lock.RLock()
	defer Lock.RUnlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return 0, ThisStream{}, false
	}

	playlist, valid := p.(Playlist)
	if !valid {
		return 0, ThisStream{}, false
	}

	for id, s := range playlist.Streams {
		if s.URL == streamingURL {
			return id, s, true
		}
	}

	return 0, ThisStream{}, false
}

// nextStreamID picks a free stream slot in a playlist.
func nextStreamID(playlistID string) int {
	Lock.RLock()
	defer Lock.RUnlock()

	p, found := BufferInformation.Load(playlistID)
	if !found {
		return 0
	}

	playlist, ok := p.(Playlist)
	if !ok {
		return 0
	}

	for i := 0; i <= len(playlist.Streams); i++ {
		if _, taken := playlist.Streams[i]; !taken {
			return i
		}
	}

	return len(playlist.Streams)
}
