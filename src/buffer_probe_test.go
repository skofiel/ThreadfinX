package src

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestProbeDistinguishesGoneFromUnreachable is the difference the audit was
// about: a channel the provider has rotated away answers 404 and must fail
// immediately, while a backend that is not answering at all is a different
// condition and must not be mistaken for it.
func TestProbeDistinguishesGoneFromUnreachable(t *testing.T) {
	t.Run("redirect means the channel is still published", func(t *testing.T) {
		// m3u-editor answers 302 towards the provider. The probe must stop
		// here and never open a connection to the provider itself.
		var followed bool

		provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			followed = true
		}))
		defer provider.Close()

		hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, provider.URL, http.StatusFound)
		}))
		defer hop.Close()

		result := probeStreamOrigin(hop.URL+"/live/admin/movistar/316503", "Threadfin")

		if !result.Reachable || result.Gone {
			t.Fatalf("a redirect must count as published, got %s", result)
		}

		if followed {
			t.Fatal("the probe followed the redirect and opened a connection to the provider; " +
				"that consumes one of the provider's connection slots")
		}
	})

	t.Run("404 means the id was rotated away", func(t *testing.T) {
		hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer hop.Close()

		result := probeStreamOrigin(hop.URL+"/live/admin/movistar/316503", "Threadfin")

		if !result.Gone {
			t.Fatalf("a 404 must be reported as gone, got %s", result)
		}
	})

	t.Run("no listener means unreachable, not gone", func(t *testing.T) {
		dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := dead.URL
		dead.Close()

		result := probeStreamOrigin(url+"/live/admin/movistar/316503", "Threadfin")

		if result.Reachable {
			t.Fatalf("a closed backend must not look reachable, got %s", result)
		}

		if result.Gone {
			t.Fatal("an unreachable backend must not be reported as gone; it may come back")
		}
	})

	t.Run("a server error is neither gone nor unreachable", func(t *testing.T) {
		hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "upstream exploded", http.StatusBadGateway)
		}))
		defer hop.Close()

		result := probeStreamOrigin(hop.URL, "Threadfin")

		if !result.Reachable || result.Gone {
			t.Fatalf("a 502 is a transient upstream failure, got %s", result)
		}
	})

	t.Run("non-http schemes are left alone", func(t *testing.T) {
		result := probeStreamOrigin("udp://@239.0.0.1:1234", "Threadfin")

		if !result.Reachable || result.Gone {
			t.Fatalf("there is nothing cheap to probe on a multicast URL, got %s", result)
		}
	})
}

// TestSweepReleasesOrphanedBuffers: a stream folder with no registered stream
// is RAM nobody will ever reclaim, since the buffer is a memfs.
func TestSweepReleasesOrphanedBuffers(t *testing.T) {
	initBufferVFS()

	restore := System.Folder.Temp
	t.Cleanup(func() { System.Folder.Temp = restore })

	System.Folder.Temp = "/tmp/threadfinx-sweep/"

	const (
		playlistID = "M4YKP42H8O5GZ4B236G6"
		liveMD5    = "live00000000000000000000000000000"
		orphanMD5  = "orphan000000000000000000000000000"
	)

	var (
		playlistFolder = System.Folder.Temp + playlistID + string(os.PathSeparator)
		liveFolder     = playlistFolder + liveMD5 + string(os.PathSeparator)
		orphanFolder   = playlistFolder + orphanMD5 + string(os.PathSeparator)
	)

	for _, folder := range []string{liveFolder, orphanFolder} {
		if err := bufferVFS.MkdirAll(folder, 0755); err != nil {
			t.Fatal(err)
		}
		writeSegment(t, folder, "1.ts", 4096)
	}

	registerPlaylist(Playlist{
		PlaylistID: playlistID,
		Folder:     playlistFolder,
		Clients:    make(map[int]ThisClient),
		Streams:    make(map[int]ThisStream),
	})
	t.Cleanup(func() { BufferInformation.Delete(playlistID) })

	addStream(playlistID, 0, ThisStream{MD5: liveMD5, Folder: liveFolder})

	sweepOrphanedBuffers()

	if _, err := bufferVFS.Stat(liveFolder); fsIsNotExistErr(err) {
		t.Fatal("the sweep removed a folder belonging to a live stream")
	}

	if _, err := bufferVFS.Stat(orphanFolder); !fsIsNotExistErr(err) {
		t.Fatal("the orphaned folder was not released; it is RAM nothing will reclaim")
	}
}
