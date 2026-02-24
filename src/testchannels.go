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

	// Collect all channels from mappings
	var channels []TestChannelResult

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

		showInfo("Test Channels:Starting channel test process")

		for i := range channels {
			// Check for cancellation
			select {
			case <-ctx.Done():
				showInfo("Test Channels:Process cancelled")
				return
			default:
			}

			channelURL := channels[i].URL

			// Validate URL
			parsedURL, err := url.Parse(channelURL)
			if err != nil || parsedURL.Scheme == "" {
				testChannelsMutex.Lock()
				testChannelsProgress.Results[i].Status = "error"
				testChannelsProgress.Results[i].Error = "Invalid URL"
				testChannelsProgress.Failed++
				testChannelsProgress.Tested++
				testChannelsMutex.Unlock()
				continue
			}

			allowedSchemes := map[string]bool{"http": true, "https": true, "rtsp": true, "rtp": true, "udp": true}
			if !allowedSchemes[strings.ToLower(parsedURL.Scheme)] {
				testChannelsMutex.Lock()
				testChannelsProgress.Results[i].Status = "skipped"
				testChannelsProgress.Results[i].Error = "Unsupported scheme"
				testChannelsProgress.Skipped++
				testChannelsProgress.Tested++
				testChannelsMutex.Unlock()
				continue
			}

			// Run ffprobe with timeout
			probeCtx, probeCancel := context.WithTimeout(ctx, 10*time.Second)
			cmd := exec.CommandContext(probeCtx, ffprobePath, "-v", "error", "-show_streams", "-of", "json", "-timeout", "5000000", parsedURL.String())
			_, err = cmd.Output()
			probeCancel()

			testChannelsMutex.Lock()
			testChannelsProgress.Tested++
			if err != nil {
				testChannelsProgress.Results[i].Status = "error"
				testChannelsProgress.Results[i].Error = err.Error()
				testChannelsProgress.Failed++
			} else {
				testChannelsProgress.Results[i].Status = "ok"
				testChannelsProgress.OK++
			}
			testChannelsMutex.Unlock()

			showInfo(fmt.Sprintf("Test Channels:Tested %d/%d - %s: %s",
				testChannelsProgress.Tested, testChannelsProgress.Total,
				channels[i].ChannelName, testChannelsProgress.Results[i].Status))
		}

		showInfo(fmt.Sprintf("Test Channels:Complete - OK: %d, Failed: %d, Skipped: %d",
			testChannelsProgress.OK, testChannelsProgress.Failed, testChannelsProgress.Skipped))
	}()

	return nil
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
