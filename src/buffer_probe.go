package src

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

/*
Stream pre-flight.

Threadfin used to hand the URL straight to FFmpeg and wait. That makes two very
different failures look identical from the outside: a backend that is not
answering at all, and a backend that answers "404, no such channel". The first
deserves a retry, the second never will succeed - and it is the common one here,
because the provider rotates channel ids and Plex keeps requesting the old ones
until it refreshes its lineup.

The probe deliberately does not follow redirects. The upstream hop (m3u-editor)
answers with a 302 to the provider, so stopping at the redirect confirms the
channel still exists without opening a connection to the provider itself. That
matters: IPTV providers count concurrent connections, and a probe that consumed
one would leave FFmpeg fighting itself for the slot.
*/

// probeTimeout bounds the pre-flight check. It only ever talks to the upstream
// hop, so it can be short.
const probeTimeout = 5 * time.Second

// probeClient never follows redirects: a 3xx is the answer we want.
var probeClient = &http.Client{
	Timeout: probeTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// streamProbeResult describes what the upstream hop said about a channel.
type streamProbeResult struct {
	// Reachable is false when nothing answered at all.
	Reachable bool
	// Gone is true for a definitive rejection (404/410): the channel id no
	// longer exists and retrying it is pointless.
	Gone bool
	// StatusCode is 0 when nothing answered.
	StatusCode int
	// Err carries the transport error, if any.
	Err error
}

func (p streamProbeResult) String() string {
	switch {
	case !p.Reachable:
		return fmt.Sprintf("no response (%v)", p.Err)
	case p.Gone:
		return fmt.Sprintf("channel is gone (HTTP %d)", p.StatusCode)
	default:
		return fmt.Sprintf("HTTP %d", p.StatusCode)
	}
}

// probeStreamOrigin asks the upstream hop whether a channel is still there.
// Non-HTTP schemes are reported as reachable: there is nothing cheap to check.
func probeStreamOrigin(url, userAgent string) streamProbeResult {
	var lower = strings.ToLower(url)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return streamProbeResult{Reachable: true}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return streamProbeResult{Err: err}
	}

	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	// Ask for a single byte: enough to get a status line, small enough that a
	// server which ignores Range does not start pushing a stream at us.
	req.Header.Set("Range", "bytes=0-0")

	resp, err := probeClient.Do(req)
	if err != nil {
		return streamProbeResult{Err: err}
	}

	resp.Body.Close()

	return streamProbeResult{
		Reachable:  true,
		Gone:       resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone,
		StatusCode: resp.StatusCode,
	}
}
