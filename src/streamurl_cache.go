package src

import (
	"encoding/json"
	"sync"
)

/*
Streaming URL cache.

Every channel Threadfin hands to Plex points at /stream/<urlID>, where urlID is
MD5(playlistID + "-" + provider URL). getStreamInfo resolves that id back to the
provider URL when the player asks for the channel.

Two problems used to meet here.

Clearing the map at the start of an XEPG rebuild (which is what upstream still
does) made getStreamInfo return 404 for every channel while the rebuild ran, so
all active players disconnected - the periodic "rewinds". Removing the clear
(05ae5d6) fixed that but left the map append-only: when the provider rotates its
channel ids, the entry for the retired URL stays resolvable forever and
Threadfin keeps launching FFmpeg against a dead URL, which is EC 4006 and
"Stream ends prematurely".

So a rebuild now stages its entries in a second map and swaps it in atomically
when it finishes. Lookups keep resolving against the live map for the whole
rebuild - no 404 window - and entries the provider no longer publishes disappear
the moment the swap happens.

The map is read from HTTP handlers and written from the background rebuild
goroutines, so every access goes through streamingURLsMu. Without it a rebuild
racing a channel change is a "concurrent map read and map write" fatal error,
which no recover can catch.
*/

var (
	streamingURLsMu sync.RWMutex

	// streamingURLsStaging is non-nil only while a rebuild is in flight.
	streamingURLsStaging map[string]StreamInfo
)

// liveStreamingURLsLocked returns the live map, creating it if needed. Callers
// must hold streamingURLsMu for writing.
func liveStreamingURLsLocked() map[string]StreamInfo {
	if Data.Cache.StreamingURLS == nil {
		Data.Cache.StreamingURLS = make(map[string]StreamInfo)
	}

	return Data.Cache.StreamingURLS
}

// lookupStreamingURL resolves a stream id. The live map wins, so a rebuild in
// progress never makes an existing channel unresolvable; the staging map is
// consulted for channels that only exist in the rebuild.
func lookupStreamingURL(urlID string) (StreamInfo, bool) {
	streamingURLsMu.RLock()
	defer streamingURLsMu.RUnlock()

	if info, ok := Data.Cache.StreamingURLS[urlID]; ok {
		return info, true
	}

	if streamingURLsStaging != nil {
		info, ok := streamingURLsStaging[urlID]
		return info, ok
	}

	return StreamInfo{}, false
}

// storeStreamingURL records a stream id. During a rebuild the entry goes into
// both maps: into staging so it survives the swap, and into the live map so a
// player that follows the link immediately can still resolve it.
func storeStreamingURL(urlID string, info StreamInfo) {
	streamingURLsMu.Lock()
	defer streamingURLsMu.Unlock()

	liveStreamingURLsLocked()[urlID] = info

	if streamingURLsStaging != nil {
		streamingURLsStaging[urlID] = info
	}
}

// beginStreamingURLRebuild starts collecting the entries a rebuild produces.
func beginStreamingURLRebuild() {
	streamingURLsMu.Lock()
	defer streamingURLsMu.Unlock()

	streamingURLsStaging = make(map[string]StreamInfo, len(Data.Cache.StreamingURLS))
}

// commitStreamingURLRebuild swaps the staged entries in and reports how many
// stale ids were dropped. It returns a snapshot safe to serialise outside the
// lock. A commit without a matching begin is a no-op that just snapshots, so a
// failed rebuild cannot wipe the cache.
func commitStreamingURLRebuild() (snapshot map[string]StreamInfo, dropped int) {
	streamingURLsMu.Lock()

	if streamingURLsStaging != nil {
		for id := range Data.Cache.StreamingURLS {
			if _, ok := streamingURLsStaging[id]; !ok {
				dropped++
			}
		}

		Data.Cache.StreamingURLS = streamingURLsStaging
		streamingURLsStaging = nil
	}

	snapshot = make(map[string]StreamInfo, len(Data.Cache.StreamingURLS))
	for id, info := range Data.Cache.StreamingURLS {
		snapshot[id] = info
	}

	streamingURLsMu.Unlock()

	return snapshot, dropped
}

// abortStreamingURLRebuild discards staged entries, leaving the live map alone.
func abortStreamingURLRebuild() {
	streamingURLsMu.Lock()
	defer streamingURLsMu.Unlock()

	streamingURLsStaging = nil
}

// snapshotStreamingURLs copies the live map for serialisation.
func snapshotStreamingURLs() map[string]StreamInfo {
	streamingURLsMu.RLock()
	defer streamingURLsMu.RUnlock()

	var snapshot = make(map[string]StreamInfo, len(Data.Cache.StreamingURLS))
	for id, info := range Data.Cache.StreamingURLS {
		snapshot[id] = info
	}

	return snapshot
}

// resetStreamingURLCache empties the cache. Used when the playlist files
// themselves change, where keeping the old ids would be wrong.
func resetStreamingURLCache() {
	streamingURLsMu.Lock()
	defer streamingURLsMu.Unlock()

	Data.Cache.StreamingURLS = make(map[string]StreamInfo)
	streamingURLsStaging = nil
}

// loadStreamingURLsFromDisk populates the cache from urls.json. It is a no-op
// if the cache already holds entries.
func loadStreamingURLsFromDisk() error {
	streamingURLsMu.Lock()
	defer streamingURLsMu.Unlock()

	if len(Data.Cache.StreamingURLS) > 0 {
		return nil
	}

	tmp, err := loadJSONFileToMap(System.File.URLS)
	if err != nil {
		return err
	}

	var loaded = make(map[string]StreamInfo)
	if err := json.Unmarshal([]byte(mapToJSON(tmp)), &loaded); err != nil {
		return err
	}

	Data.Cache.StreamingURLS = loaded

	return nil
}
