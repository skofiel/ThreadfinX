package src

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

/*
Stream buffer guard.

The buffer lives in a memfs virtual filesystem. memfs does not implement POSIX
unlink semantics: when the last link to a file goes away it frees the node's
backing slice immediately, even if another goroutine still holds the file open.
A reader that is part way through the file then evaluates data[offset:] on a nil
slice and panics with "slice bounds out of range [offset:0]".

That is not hypothetical - it is the production crash this guard exists to stop.
The producer (the FFmpeg goroutine) tears the stream folder down while a
consumer (an HTTP handler) is still copying a segment out of it.

The guard serialises the two sides per stream folder:

  - readers take the read lock for the (in-memory, microsecond) copy of one
    segment into a buffer, and release it before writing anything to the client,
    so a slow or wedged player can never hold teardown hostage;
  - anything that unlinks - folder teardown and old-segment rotation - takes the
    write lock, and so waits for in-flight readers instead of pulling the data
    out from under them.

Once a folder is closed the guard stays closed, so a reader that was waiting on
the lock gets errStreamGone rather than an empty file it would mistake for EOF.
*/

// errStreamGone is returned when the stream buffer was torn down while a reader
// was waiting for it. Callers should treat it as "this stream is over", not as
// an I/O error worth retrying.
var errStreamGone = errors.New("stream buffer is no longer available")

type streamGuard struct {
	mu     sync.RWMutex
	closed bool
}

var (
	streamGuardsMu sync.Mutex
	streamGuards   = make(map[string]*streamGuard)
)

// streamGuardFor returns the guard for a stream folder, creating it on demand.
// A folder that was torn down gets a fresh (open) guard: the reads through it
// will simply fail to open their files, which is the correct outcome, and a
// stream later restarted on the same folder is not poisoned by the old guard.
func streamGuardFor(folder string) *streamGuard {
	streamGuardsMu.Lock()
	defer streamGuardsMu.Unlock()

	if g, ok := streamGuards[folder]; ok {
		return g
	}

	g := &streamGuard{}
	streamGuards[folder] = g

	return g
}

// readSegment copies a complete segment into *bufPtr while holding the read
// lock, growing the buffer if the segment does not fit. It returns the number
// of bytes read; the caller writes them to the client after the guard has been
// released.
func (g *streamGuard) readSegment(fileName string, bufPtr *[]byte) (n int, err error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.closed {
		return 0, errStreamGone
	}

	file, err := bufferVFS.Open(fileName)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	var size = int(info.Size())
	if size <= 0 {
		return 0, nil
	}

	if cap(*bufPtr) < size {
		*bufPtr = make([]byte, size)
	}
	var buf = (*bufPtr)[:size]

	for n < size {
		m, readErr := file.Read(buf[n:])
		n += m

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return n, readErr
		}

		if m == 0 {
			break
		}
	}

	return n, nil
}

// closeStreamBuffer removes a stream folder and everything in it, waiting for
// in-flight readers first. Every teardown of a stream folder must go through
// here - a bare bufferVFS.RemoveAll is the crash.
func closeStreamBuffer(folder string) error {
	g := streamGuardFor(folder)

	g.mu.Lock()
	g.closed = true
	err := bufferVFS.RemoveAll(getPlatformPath(folder))
	g.mu.Unlock()

	streamGuardsMu.Lock()
	// Only drop our own guard: a stream restarted on this folder may already
	// have registered a fresh one.
	if current, ok := streamGuards[folder]; ok && current == g {
		delete(streamGuards, folder)
	}
	streamGuardsMu.Unlock()

	return err
}

// removeStreamSegment unlinks a single expired segment, again waiting for
// readers. Called by the producer as it rotates the buffer window.
func removeStreamSegment(folder, name string) error {
	g := streamGuardFor(folder)

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return nil
	}

	return bufferVFS.RemoveAll(getPlatformFile(folder + name))
}

// inactivityTimeout is how long the buffer waits for data from FFmpeg/VLC
// before declaring the stream dead. Configurable via buffer.inactivity.timeout;
// a non-positive value disables the watchdog.
func inactivityTimeout() time.Duration {
	systemMutex.Lock()
	var seconds = Settings.BufferInactivityTimeout
	systemMutex.Unlock()

	if seconds <= 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

// liveBufferFolders returns the folder of every stream currently registered.
func liveBufferFolders() map[string]bool {
	var live = make(map[string]bool)

	Lock.RLock()
	defer Lock.RUnlock()

	BufferInformation.Range(func(_, value interface{}) bool {
		playlist, ok := value.(Playlist)
		if !ok {
			return true
		}

		for _, stream := range playlist.Streams {
			if stream.Folder != "" {
				live[stream.Folder] = true
			}
		}

		return true
	})

	return live
}

// sweepOrphanedBuffers frees buffer folders that no longer belong to any
// registered stream.
//
// The buffer lives in RAM (memfs), roughly 10 MB per stream at the default
// buffer size, and a folder is normally freed by clientConnection when the last
// viewer leaves. Anything that ends a stream without going through that path -
// a panic in the handler, a producer goroutine that never returns - used to
// leak the folder until Threadfin was restarted. On a Pi that is expected to
// run for weeks, that adds up.
func sweepOrphanedBuffers() {
	if _, err := bufferVFS.Stat(System.Folder.Temp); fsIsNotExistErr(err) {
		return
	}

	var live = liveBufferFolders()

	playlists, err := bufferVFS.ReadDir(getPlatformPath(System.Folder.Temp))
	if err != nil {
		return
	}

	for _, playlist := range playlists {
		if !playlist.IsDir() {
			continue
		}

		var playlistFolder = System.Folder.Temp + playlist.Name() + string(os.PathSeparator)

		streams, err := bufferVFS.ReadDir(getPlatformPath(playlistFolder))
		if err != nil {
			continue
		}

		for _, stream := range streams {
			if !stream.IsDir() {
				continue
			}

			var folder = playlistFolder + stream.Name() + string(os.PathSeparator)

			if live[folder] {
				continue
			}

			showDebug(fmt.Sprintf("Buffer Status:Releasing orphaned buffer folder %s", folder), 1)

			if err := closeStreamBuffer(folder); err != nil {
				ShowError(err, 4005)
			}
		}
	}
}
