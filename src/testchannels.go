package src

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TestChannelResult contains the result of testing a single channel
type TestChannelResult struct {
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
	URL         string `json:"url"`
	Active      bool   `json:"active"`
	Status      string `json:"status"` // "ok", "error", "skipped"
	Error       string `json:"error,omitempty"`
}

// TestChannelsProgress contains the current progress of the test
type TestChannelsProgress struct {
	Running  bool                `json:"running"`
	Total    int                 `json:"total"`
	Tested   int                 `json:"tested"`
	OK       int                 `json:"ok"`
	Failed   int                 `json:"failed"`
	Skipped  int                 `json:"skipped"`
	Results  []TestChannelResult `json:"results"`
	Complete bool                `json:"complete"`
}

var (
	testChannelsProgress TestChannelsProgress
	testChannelsMutex    sync.Mutex
	testChannelsRunning  bool
	testChannelsCancel   context.CancelFunc
)

// StartTestChannels begins the channel testing process
func StartTestChannels() error {
	testChannelsMutex.Lock()
	if testChannelsRunning {
		testChannelsMutex.Unlock()
		return fmt.Errorf("test channels process is already running")
	}

	// Check if ffprobe is available
	ffmpegPath := Settings.FFmpegPath
	if ffmpegPath == "" {
		testChannelsMutex.Unlock()
		return fmt.Errorf("FFmpeg path not configured - required for channel testing")
	}

	ffprobePath := strings.Replace(ffmpegPath, "ffmpeg", "ffprobe", 1)

	// Collect all channels from mappings. Data.XEPG.Channels is rebuilt by the
	// background XEPG goroutines, so the walk needs the same lock they take.
	var channels []TestChannelResult

	xepgMutex.Lock()
	for id, xepgChannel := range Data.XEPG.Channels {
		channelMap, ok := xepgChannel.(map[string]interface{})
		if !ok {
			continue
		}

		channelURL, _ := channelMap["url"].(string)
		channelName, _ := channelMap["x-name"].(string)
		active, _ := channelMap["x-active"].(bool)

		if channelURL == "" {
			continue
		}

		channels = append(channels, TestChannelResult{
			ChannelID:   id,
			ChannelName: channelName,
			URL:         channelURL,
			Active:      active,
			Status:      "pending",
		})
	}
	xepgMutex.Unlock()

	if len(channels) == 0 {
		testChannelsMutex.Unlock()
		return fmt.Errorf("no channels found to test")
	}

	// Initialize progress
	testChannelsProgress = TestChannelsProgress{
		Running:  true,
		Total:    len(channels),
		Tested:   0,
		OK:       0,
		Failed:   0,
		Skipped:  0,
		Results:  channels,
		Complete: false,
	}
	testChannelsRunning = true

	ctx, cancel := context.WithCancel(context.Background())
	testChannelsCancel = cancel
	testChannelsMutex.Unlock()

	// Run test in background goroutine
	go func() {
		defer func() {
			testChannelsMutex.Lock()
			testChannelsProgress.Running = false
			testChannelsProgress.Complete = true
			testChannelsRunning = false
			testChannelsMutex.Unlock()
		}()

		showInfo(fmt.Sprintf("Test Channels:Starting channel test process (%d channels, %d at a time)",
			len(channels), testChannelsWorkers))

		// One ffprobe at a time meant hours for a few hundred channels. Keep
		// the pool small all the same: every probe is a connection to the
		// provider, and providers count those.
		var (
			wg    sync.WaitGroup
			queue = make(chan int)
		)

		for w := 0; w < testChannelsWorkers; w++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				for i := range queue {
					testOneChannel(ctx, ffprobePath, i, channels[i].URL, channels[i].ChannelName)
				}
			}()
		}

	dispatch:
		for i := range channels {
			select {
			case <-ctx.Done():
				showInfo("Test Channels:Process cancelled")
				break dispatch
			case queue <- i:
			}
		}

		close(queue)
		wg.Wait()

		showInfo(fmt.Sprintf("Test Channels:Complete - OK: %d, Failed: %d, Skipped: %d",
			testChannelsProgress.OK, testChannelsProgress.Failed, testChannelsProgress.Skipped))
	}()

	return nil
}

// testChannelsWorkers bounds how many ffprobe processes run at once. Each one
// is a connection to the provider, so this stays deliberately low.
const testChannelsWorkers = 4

// testChannelProbeTimeout bounds a single probe. A channel that has not
// answered in this long is not going to.
const testChannelProbeTimeout = 5 * time.Second

// testOneChannel probes a single channel and records the outcome.
func testOneChannel(ctx context.Context, ffprobePath string, index int, channelURL, channelName string) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	var record = func(status, message string) {
		testChannelsMutex.Lock()
		defer testChannelsMutex.Unlock()

		testChannelsProgress.Results[index].Status = status
		testChannelsProgress.Results[index].Error = message
		testChannelsProgress.Tested++

		switch status {
		case "ok":
			testChannelsProgress.OK++
		case "skipped":
			testChannelsProgress.Skipped++
		default:
			testChannelsProgress.Failed++
		}

		showInfo(fmt.Sprintf("Test Channels:Tested %d/%d - %s: %s",
			testChannelsProgress.Tested, testChannelsProgress.Total, channelName, status))
	}

	parsedURL, err := url.Parse(channelURL)
	if err != nil || parsedURL.Scheme == "" {
		record("error", "Invalid URL")
		return
	}

	allowedSchemes := map[string]bool{"http": true, "https": true, "rtsp": true, "rtp": true, "udp": true}
	if !allowedSchemes[strings.ToLower(parsedURL.Scheme)] {
		record("skipped", "Unsupported scheme")
		return
	}

	probeCtx, probeCancel := context.WithTimeout(ctx, testChannelProbeTimeout)
	defer probeCancel()

	cmd := exec.CommandContext(probeCtx, ffprobePath, "-v", "error", "-show_streams", "-of", "json",
		"-timeout", "5000000", parsedURL.String())

	if _, err := cmd.Output(); err != nil {
		record("error", err.Error())
		return
	}

	record("ok", "")
}

// StopTestChannels cancels the running test
func StopTestChannels() {
	testChannelsMutex.Lock()
	defer testChannelsMutex.Unlock()
	if testChannelsCancel != nil {
		testChannelsCancel()
	}
}

// GetTestChannelsProgress returns the current progress
func GetTestChannelsProgress() TestChannelsProgress {
	testChannelsMutex.Lock()
	defer testChannelsMutex.Unlock()
	return testChannelsProgress
}
