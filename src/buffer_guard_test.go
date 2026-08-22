package src

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/avfs/avfs/vfs/memfs"
)

// recordingWriter is a minimal http.ResponseWriter that counts what a client
// received. writeDelay simulates a player that reads slowly, which is what
// makes the teardown race observable.
type recordingWriter struct {
	mu         sync.Mutex
	bytes      int
	header     http.Header
	writeDelay time.Duration
}

func newRecordingWriter(delay time.Duration) *recordingWriter {
	return &recordingWriter{header: make(http.Header), writeDelay: delay}
}

func (r *recordingWriter) Header() http.Header { return r.header }
func (r *recordingWriter) WriteHeader(int)     {}

func (r *recordingWriter) Write(b []byte) (int, error) {
	if r.writeDelay > 0 {
		time.Sleep(r.writeDelay)
	}

	r.mu.Lock()
	r.bytes += len(b)
	r.mu.Unlock()

	return len(b), nil
}

func (r *recordingWriter) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.bytes
}

// writeSegment puts a segment of the given size into the buffer filesystem.
func writeSegment(t *testing.T, folder, name string, size int) {
	t.Helper()

	f, err := bufferVFS.OpenFile(folder+name, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("create segment %s: %v", name, err)
	}

	if _, err := f.Write(make([]byte, size)); err != nil {
		t.Fatalf("write segment %s: %v", name, err)
	}

	f.Close()
}

func newTestBuffer(t *testing.T, folder string, segments, size int) {
	t.Helper()

	initBufferVFS()

	if err := bufferVFS.MkdirAll(folder, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", folder, err)
	}

	for i := 1; i <= segments; i++ {
		writeSegment(t, folder, fmt.Sprintf("%d.ts", i), size)
	}
}

// TestMemFSFreesDataUnderOpenReaders is a characterisation test for the
// vendored avfs/memfs, and the reason buffer_guard.go exists.
//
// memfs does not implement POSIX unlink semantics: removing a file frees the
// node's backing slice immediately, even though a reader still holds it open.
// The reader's next Read then evaluates data[offset:] on a nil slice.
//
// This is the exact production crash:
//
//	runtime error: slice bounds out of range [557568:0]
//	memfs.(*MemFile).Read()  memfs_file.go:220
//
// If this test ever FAILS, that is good news: the dependency changed its
// semantics and the guard around every unlink could be reconsidered.
func TestMemFSFreesDataUnderOpenReaders(t *testing.T) {
	vfs := memfs.New()

	if err := vfs.MkdirAll("/buf", 0755); err != nil {
		t.Fatal(err)
	}

	f, err := vfs.OpenFile("/buf/1.ts", os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(make([]byte, 1024*1024))
	f.Close()

	reader, err := vfs.Open("/buf/1.ts")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	// Read part of the file, so the handle's offset is non-zero. An offset of
	// zero would survive: nil[0:] is a legal empty slice.
	if _, err := reader.Read(make([]byte, 32*1024)); err != nil {
		t.Fatalf("first read: %v", err)
	}

	if err := vfs.RemoveAll("/buf"); err != nil {
		t.Fatal(err)
	}

	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				t.Logf("reproduced the production panic: %v", r)
			}
		}()

		reader.Read(make([]byte, 32*1024))
	}()

	if !panicked {
		t.Fatal("memfs no longer frees file data under an open reader; " +
			"the unlink guard in buffer_guard.go may no longer be required")
	}
}

// removeOnWriteWriter unlinks the buffer folder as soon as the first chunk has
// been written, which is precisely what the producer does to a slow client.
type removeOnWriteWriter struct {
	folder string
	once   sync.Once
	header http.Header
}

func (r *removeOnWriteWriter) Header() http.Header { return r.header }
func (r *removeOnWriteWriter) WriteHeader(int)     {}

func (r *removeOnWriteWriter) Write(b []byte) (int, error) {
	r.once.Do(func() { bufferVFS.RemoveAll(r.folder) })
	return len(b), nil
}

// TestStreamingDirectlyFromBufferFilePanics pins down why sendSegmentToClient
// reads a whole segment into memory instead of streaming it straight out of the
// virtual filesystem.
//
// The pre-fix implementation was io.CopyBuffer(w, file, buf): a loop of Reads
// against a handle held open for the whole transfer, with a slow client between
// the iterations. Unlinking the folder part way through that loop is the
// production crash. Reading the segment in one shot from offset zero cannot
// panic - nil[0:] is a legal empty slice - which is why the current
// implementation copies first and writes afterwards.
//
// If this test stops panicking the dependency changed; see
// TestMemFSFreesDataUnderOpenReaders.
func TestStreamingDirectlyFromBufferFilePanics(t *testing.T) {
	const folder = "/tmp/threadfinx-test/direct-copy/"

	newTestBuffer(t, folder, 1, 1024*1024)

	file, err := bufferVFS.Open(folder + "1.ts")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var panicked bool

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				t.Logf("the pre-fix copy path panics as in production: %v", r)
			}
		}()

		w := &removeOnWriteWriter{folder: folder, header: make(http.Header)}
		buf := make([]byte, 32*1024)
		io.CopyBuffer(w, file, buf)
	}()

	if !panicked {
		t.Fatal("streaming straight from the VFS handle no longer panics on unlink; " +
			"the copy-then-write split in sendSegmentToClient may be reconsidered")
	}
}

// TestCloseStreamBufferWaitsForReaders is the contract test for the guard: an
// unlink must block until in-flight readers are done. It fails if the locking
// in buffer_guard.go is weakened or removed.
func TestCloseStreamBufferWaitsForReaders(t *testing.T) {
	const folder = "/tmp/threadfinx-test/wait/"

	newTestBuffer(t, folder, 2, 64*1024)

	guard := streamGuardFor(folder)

	// Stand in for a reader that is part way through a segment.
	guard.mu.RLock()

	done := make(chan error, 1)
	go func() { done <- closeStreamBuffer(folder) }()

	select {
	case <-done:
		guard.mu.RUnlock()
		t.Fatal("closeStreamBuffer tore the buffer down while a reader held it")
	case <-time.After(150 * time.Millisecond):
	}

	guard.mu.RUnlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("closeStreamBuffer: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("closeStreamBuffer never completed after the reader released")
	}

	if _, err := bufferVFS.Stat(folder); !fsIsNotExistErr(err) {
		t.Fatal("buffer folder survived teardown")
	}
}

// TestRemoveStreamSegmentWaitsForReaders is the same contract for the rotation
// path the producer uses.
func TestRemoveStreamSegmentWaitsForReaders(t *testing.T) {
	const folder = "/tmp/threadfinx-test/wait-rotation/"

	newTestBuffer(t, folder, 2, 64*1024)

	guard := streamGuardFor(folder)
	guard.mu.RLock()

	done := make(chan error, 1)
	go func() { done <- removeStreamSegment(folder, "1.ts") }()

	select {
	case <-done:
		guard.mu.RUnlock()
		t.Fatal("removeStreamSegment unlinked a segment while a reader held the guard")
	case <-time.After(150 * time.Millisecond):
	}

	guard.mu.RUnlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("removeStreamSegment: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("removeStreamSegment never completed")
	}
}

// TestReadAfterTeardownIsAnError checks that a client racing teardown is told
// the stream is gone instead of silently receiving a zero-length segment, which
// would look like a healthy but empty read.
func TestReadAfterTeardownIsAnError(t *testing.T) {
	const folder = "/tmp/threadfinx-test/gone/"

	newTestBuffer(t, folder, 1, 64*1024)

	guard := streamGuardFor(folder)

	if err := closeStreamBuffer(folder); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 0)
	if _, err := guard.readSegment(folder+"1.ts", &buf); err != errStreamGone {
		t.Fatalf("want errStreamGone after teardown, got %v", err)
	}
}

// TestStreamGuardSurvivesTeardownDuringRead drives sendSegmentToClient and
// closeStreamBuffer against each other, which is the pair of functions that
// crashed in production.
func TestStreamGuardSurvivesTeardownDuringRead(t *testing.T) {
	const folder = "/tmp/threadfinx-test/guard/"

	newTestBuffer(t, folder, 40, 512*1024)

	var (
		wg       sync.WaitGroup
		panics   atomic.Int32
		streamed atomic.Int64
	)

	// Several clients on the same stream, as happens when Plex opens a second
	// connection while the first is still draining.
	for c := 0; c < 4; c++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
					t.Errorf("panic in client goroutine: %v", r)
				}
			}()

			var (
				debug string
				w     = newRecordingWriter(time.Millisecond)
			)

			for i := 1; i <= 40; i++ {
				name := fmt.Sprintf("%s%d.ts", folder, i)
				if err := sendSegmentToClient(folder, name, w, &debug); err != nil {
					// errStreamGone / "file does not exist" are the correct
					// outcomes once teardown has happened.
					break
				}
			}

			streamed.Add(int64(w.total()))
		}()
	}

	// Meanwhile the producer decides the stream is over and tears it down.
	time.Sleep(5 * time.Millisecond)

	if err := closeStreamBuffer(folder); err != nil {
		t.Fatalf("closeStreamBuffer: %v", err)
	}

	wg.Wait()

	if panics.Load() != 0 {
		t.Fatalf("%d client goroutines panicked", panics.Load())
	}

	if streamed.Load() == 0 {
		t.Fatal("no data reached any client; the test did not exercise the race")
	}

	if _, err := bufferVFS.Stat(folder); !fsIsNotExistErr(err) {
		t.Fatal("buffer folder survived teardown")
	}
}

// TestStreamGuardSurvivesSegmentRotation covers the second unlink path: the
// producer retiring an expired segment while a client reads it.
func TestStreamGuardSurvivesSegmentRotation(t *testing.T) {
	const folder = "/tmp/threadfinx-test/rotation/"

	newTestBuffer(t, folder, 30, 512*1024)

	var (
		wg     sync.WaitGroup
		panics atomic.Int32
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				panics.Add(1)
				t.Errorf("panic while reading a rotating buffer: %v", r)
			}
		}()

		var (
			debug string
			w     = newRecordingWriter(0)
		)

		for round := 0; round < 5; round++ {
			for i := 1; i <= 30; i++ {
				name := fmt.Sprintf("%s%d.ts", folder, i)
				sendSegmentToClient(folder, name, w, &debug)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 1; i <= 30; i++ {
			if err := removeStreamSegment(folder, fmt.Sprintf("%d.ts", i)); err != nil {
				t.Errorf("removeStreamSegment: %v", err)
				return
			}
		}
	}()

	wg.Wait()

	if panics.Load() != 0 {
		t.Fatalf("%d goroutines panicked during rotation", panics.Load())
	}
}

// TestBackendCutMidStreamDoesNotPanic reproduces the production sequence end to
// end, using the real functions on both sides:
//
//	provider stops feeding  ->  FFmpeg goroutine gives up
//	                        ->  clientConnection() sees no registered clients
//	                        ->  it removes the stream folder
//	                        ->  meanwhile an HTTP handler is still copying a segment
//
// Before the guard this reliably produced
// "slice bounds out of range [N:0]" inside net/http's connection goroutine.
func TestBackendCutMidStreamDoesNotPanic(t *testing.T) {
	const (
		playlistID = "M4YKP42H8O5GZ4B236G6"
		md5        = "6f34ad036afe0738b9c7e2f165060a42"
		folder     = "/tmp/threadfinx-test/backend-cut/"
	)

	newTestBuffer(t, folder, 30, 512*1024)

	stream := ThisStream{
		ChannelName: "M+ LaLiga",
		Folder:      folder,
		MD5:         md5,
		PlaylistID:  playlistID,
	}

	// A client is registered and watching.
	BufferClients.Store(playlistID+md5, ClientConnection{Connection: 1})
	defer BufferClients.Delete(playlistID + md5)

	var (
		clientDone = make(chan struct{})
		panicked   atomic.Bool
		lastErr    atomic.Value
	)

	// The HTTP handler side: copying segments to a slow player.
	go func() {
		defer close(clientDone)
		defer func() {
			if r := recover(); r != nil {
				panicked.Store(true)
				t.Errorf("panic serving client: %v", r)
			}
		}()

		var (
			debug string
			w     = newRecordingWriter(2 * time.Millisecond)
		)

		for i := 1; i <= 30; i++ {
			name := fmt.Sprintf("%s%d.ts", folder, i)
			if err := sendSegmentToClient(folder, name, w, &debug); err != nil {
				lastErr.Store(err.Error())
				return
			}
		}
	}()

	// The producer side: the backend cut, the last client record is gone, and
	// the FFmpeg goroutine runs its end-of-stream check.
	time.Sleep(10 * time.Millisecond)
	BufferClients.Delete(playlistID + md5)

	if clientConnection(stream) {
		t.Fatal("clientConnection reported the stream as still in use")
	}

	select {
	case <-clientDone:
	case <-time.After(30 * time.Second):
		t.Fatal("client goroutine did not finish; teardown is blocked on a reader")
	}

	if panicked.Load() {
		t.Fatal("client goroutine panicked on backend cut")
	}

	// The client must be told the stream is over, not left hanging.
	if lastErr.Load() == nil {
		t.Fatal("client finished without observing the teardown")
	}

	t.Logf("client stopped cleanly with: %v", lastErr.Load())

	if _, err := bufferVFS.Stat(folder); !fsIsNotExistErr(err) {
		t.Fatal("buffer folder was not removed")
	}
}
