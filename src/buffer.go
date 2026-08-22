package src

/*
  Render tuner-limit image as video [ffmpeg]
  -loop 1 -i stream-limit.jpg -c:v libx264 -t 1 -pix_fmt yuv420p -vf scale=1920:1080  stream-limit.ts
*/

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/avfs/avfs/vfs/memfs"
)

// bufferedSegments is how many finished segments are kept in the buffer folder
// before the oldest is retired. At the default buffer size that is roughly ten
// seconds of stream.
const bufferedSegments = 21

// copyBufPool holds reusable segment-sized byte slices. readSegment grows them
// to fit the largest segment it has seen, so after the first few segments the
// streaming path stops allocating.
var copyBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 128*1024) // 128KB, grown on demand
		return &buf
	},
}

type BackupStream struct {
	PlaylistID string
	URL        string
}

// getActiveClientCount is reached from the WebUI dashboard, so it runs on an
// HTTP goroutine while streams are live. It both reads and repairs the client
// maps, which is why it has to hold Lock for the whole walk.
func getActiveClientCount() (count int) {
	Lock.Lock()
	defer Lock.Unlock()

	cleanUpStaleClientsLocked()

	BufferInformation.Range(func(key, value interface{}) bool {
		playlist, ok := value.(Playlist)
		if !ok {
			showDebug(fmt.Sprintf("Invalid type assertion for playlist: %v", value), 2)
			return true
		}

		for clientID, client := range playlist.Clients {
			if client.Connection < 0 {
				showDebug(fmt.Sprintf("Client ID %d has negative connections: %d. Resetting to 0.", clientID, client.Connection), 2)
				client.Connection = 0
				playlist.Clients[clientID] = client
				BufferInformation.Store(key, playlist)
			}
			if client.Connection > 1 {
				showDebug(fmt.Sprintf("Client ID %d has suspiciously high connections: %d. Resetting to 1.", clientID, client.Connection), 2)
				client.Connection = 1
				playlist.Clients[clientID] = client
				BufferInformation.Store(key, playlist)
			}
			count += client.Connection
		}

		showDebug(fmt.Sprintf("Playlist %s has %d active clients", playlist.PlaylistID, len(playlist.Clients)), 3)
		return true
	})

	return count
}

func getActivePlaylistCount() (count int) {
	Lock.RLock()
	defer Lock.RUnlock()

	count = 0
	BufferInformation.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// cleanUpStaleClientsLocked must be called with Lock held for writing.
func cleanUpStaleClientsLocked() {
	BufferInformation.Range(func(key, value interface{}) bool {
		playlist, ok := value.(Playlist)
		if !ok {
			showDebug(fmt.Sprintf("Invalid type assertion for playlist: %v", value), 2)
			return true
		}

		for clientID, client := range playlist.Clients {
			if client.Connection <= 0 {
				showDebug(fmt.Sprintf("Removing stale client ID %d from playlist %s", clientID, playlist.PlaylistID), 2)
				delete(playlist.Clients, clientID)
			}
		}
		BufferInformation.Store(key, playlist)
		return true
	})
}

func getClientIP(r *http.Request) string {
	// Check the X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For may contain multiple IP addresses; return the first one
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check the X-Real-IP header next
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fallback to RemoteAddr
	ip := r.RemoteAddr
	if strings.Contains(ip, ":") {
		// Remove port if present
		ip = strings.Split(ip, ":")[0]
	}

	return ip
}

func createStreamID(stream map[int]ThisStream, ip, userAgent string) (streamID int) {
	streamID = 0
	uniqueIdentifier := fmt.Sprintf("%s-%s", ip, userAgent)

	for i := 0; i <= len(stream); i++ {
		if _, ok := stream[i]; !ok {
			streamID = i
			break
		}
	}

	if _, ok := stream[streamID]; ok && stream[streamID].ClientID == uniqueIdentifier {
		// Return the same ID if the combination already exists
		return streamID
	}

	return
}

func bufferingStream(playlistID string, streamingURL string, backupStream1 *BackupStream, backupStream2 *BackupStream, backupStream3 *BackupStream, channelName string, w http.ResponseWriter, r *http.Request) {

	time.Sleep(time.Duration(Settings.BufferTimeout) * time.Millisecond)

	var playlist Playlist
	var client ThisClient
	var stream ThisStream
	var streamID int
	var debug string
	var timeOut = 0
	var newStream = true

	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check whether the playlist is already in use
	if p, ok := getPlaylistSnapshot(playlistID); !ok {
		var playlistType string

		// Playlist is not yet in use, create default values for the playlist
		playlist.Folder = System.Folder.Temp + playlistID + string(os.PathSeparator)
		playlist.PlaylistID = playlistID
		playlist.Streams = make(map[int]ThisStream)
		playlist.Clients = make(map[int]ThisClient)

		err := checkVFSFolder(playlist.Folder, bufferVFS)
		if err != nil {
			ShowError(err, 000)
			httpStatusError(w, r, 404)
			return
		}

		switch playlist.PlaylistID[0:1] {

		case "M":
			playlistType = "m3u"

		case "H":
			playlistType = "hdhr"

		}

		var playListBuffer string
		systemMutex.Lock()
		playListInterface := Settings.Files.M3U[playlistID]
		if playListInterface == nil {
			playListInterface = Settings.Files.HDHR[playlistID]
		}
		if playListMap, ok := playListInterface.(map[string]interface{}); ok {
			if buffer, ok := playListMap["buffer"].(string); ok {
				playListBuffer = buffer
			} else {
				playListBuffer = "-"
			}
		}
		systemMutex.Unlock()

		playlist.Buffer = playListBuffer

		playlist.Tuner = getTuner(playlistID, playlistType)

		playlist.PlaylistName = getProviderParameter(playlist.PlaylistID, playlistType, "name")

		playlist.HttpProxyIP = getProviderParameter(playlist.PlaylistID, playlistType, "http_proxy.ip")
		playlist.HttpProxyPort = getProviderParameter(playlist.PlaylistID, playlistType, "http_proxy.port")

		playlist.HttpUserOrigin = getProviderParameter(playlist.PlaylistID, playlistType, "http_headers.origin")
		playlist.HttpUserReferer = getProviderParameter(playlist.PlaylistID, playlistType, "http_headers.referer")

		// Create default values for the stream
		streamID = createStreamID(playlist.Streams, getClientIP(r), r.UserAgent())

		client.Connection += 1

		stream.URL = streamingURL
		stream.BackupChannel1 = backupStream1
		stream.BackupChannel2 = backupStream2
		stream.BackupChannel3 = backupStream3
		stream.ChannelName = channelName
		stream.Status = false

		playlist.Streams[streamID] = stream
		playlist.Clients[streamID] = client

		// Publish only now: until this point the maps are private to this
		// goroutine, so building them needs no lock.
		registerPlaylist(playlist)

	} else {
		playlist = p

		// Playlist is already used for streaming
		// Check if the URL is already streaming from another client.
		if id, existing, found := findStreamByURL(playlistID, streamingURL); found {

			streamID = id
			newStream = false

			stream = existing
			stream.BackupChannel1 = backupStream1
			stream.BackupChannel2 = backupStream2
			stream.BackupChannel3 = backupStream3
			stream.ChannelName = channelName
			stream.Status = false

			connections, ok := joinStream(playlistID, streamID)
			if !ok {
				// The stream went away between the lookup and the join.
				httpStatusError(w, r, 404)
				return
			}

			client.Connection = connections

			debug = fmt.Sprintf("Restream Status:Playlist: %s - Channel: %s - Connections: %d", playlist.PlaylistName, stream.ChannelName, connections)

			showDebug(debug, 1)

			if c, ok := BufferClients.Load(playlistID + stream.MD5); ok {

				var clients = c.(ClientConnection)
				clients.Connection = connections

				showInfo(fmt.Sprintf("Streaming Status:Channel: %s (Clients: %d)", stream.ChannelName, connections))

				BufferClients.Store(playlistID+stream.MD5, clients)

			}

		}

		// New stream for an already active playlist
		if newStream {

			// Check if the playlist allows another stream (Tuner)
			if getStreamCount(playlistID) >= playlist.Tuner {
				// If there are backup URLs, use them
				if backupStream1 != nil {
					bufferingStream(backupStream1.PlaylistID, backupStream1.URL, nil, backupStream2, backupStream3, channelName, w, r)
				} else if backupStream2 != nil && backupStream1 == nil {
					bufferingStream(backupStream2.PlaylistID, backupStream2.URL, nil, nil, backupStream3, channelName, w, r)
				} else if backupStream3 != nil && backupStream1 == nil && backupStream2 == nil {
					bufferingStream(backupStream3.PlaylistID, backupStream3.URL, nil, nil, nil, channelName, w, r)
				}

				showInfo(fmt.Sprintf("Streaming Status:Playlist: %s - No new connections available. Tuner = %d", playlist.PlaylistName, playlist.Tuner))

				if value, ok := webUI["html/video/stream-limit.ts"]; ok {

					content := GetHTMLString(value.(string))

					w.Header().Set("Content-Type", "video/mpeg")
					w.Header().Set("Content-Length", "0")
					w.WriteHeader(200)

					for i := 1; i < 60; i++ {
						_ = i
						w.Write([]byte(content))
						time.Sleep(time.Duration(500) * time.Millisecond)
					}

					return
				}

				return
			}

			// Playlist allows another stream (Tuner limit not yet reached)
			// Create default values for the stream
			stream = ThisStream{}
			client = ThisClient{}

			streamID = nextStreamID(playlistID)

			client.Connection = 1
			stream.URL = streamingURL
			stream.ChannelName = channelName
			stream.Status = false
			stream.BackupChannel1 = backupStream1
			stream.BackupChannel2 = backupStream2
			stream.BackupChannel3 = backupStream3

			addStream(playlistID, streamID, stream)

		}

	}

	// Check whether the stream is already being played by another client
	if current, ok := getStreamSnapshot(playlistID, streamID); ok && !current.Status && newStream {

		// New buffer is needed
		stream = current
		stream.MD5 = getMD5(streamingURL)
		stream.Folder = playlist.Folder + stream.MD5 + string(os.PathSeparator)
		stream.PlaylistID = playlistID
		stream.PlaylistName = playlist.PlaylistName
		stream.BackupChannel1 = backupStream1
		stream.BackupChannel2 = backupStream2
		stream.BackupChannel3 = backupStream3

		putStream(playlistID, streamID, stream)

		// Register the client before starting the producer. The producer's very
		// first clientConnection() check looks this entry up and tears the
		// stream down if it is missing, so publishing it afterwards was a race
		// that could kill a stream the instant it started.
		var clients ClientConnection
		clients.Connection = 1
		BufferClients.Store(playlistID+stream.MD5, clients)

		switch playlist.Buffer {

		case "ffmpeg", "vlc":
			go thirdPartyBuffer(streamID, playlistID, false, 0)

		default:
			break

		}

		showInfo(fmt.Sprintf("Streaming Status 1:Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, getStreamCount(playlistID), playlist.Tuner))

	}

	// Headers have to be set before WriteHeader; anything set afterwards is
	// silently discarded, which is what happened to the Content-Type the
	// segment writer used to compute. This is the type Go's sniffer produced
	// from MPEG-TS payload anyway, now stated rather than inferred.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(200)

	for { //Loop 1: Wait until the first segment has been downloaded through the buffer

		if playlist, ok := getPlaylistSnapshot(playlistID); ok {

			if stream, ok := getStreamSnapshot(playlistID, streamID); ok {

				if !stream.Status {

					timeOut++

					time.Sleep(time.Duration(100) * time.Millisecond)

					if c, ok := BufferClients.Load(playlistID + stream.MD5); ok {

						var clients = c.(ClientConnection)

						if clients.Error != nil || (timeOut > 200 && (stream.BackupChannel1 == nil && stream.BackupChannel2 == nil && stream.BackupChannel3 == nil)) {
							killClientConnection(streamID, stream.PlaylistID, false)
							return
						}

					}

					continue
				}

				var segmentsSinceSync int

				for { // Loop 2: Temporary files are present, data can be sent to the client

					// Monitor HTTP client connection

					ctx := r.Context()
					if ok {

						select {

						case <-ctx.Done():
							killClientConnection(streamID, playlistID, false)
							return

						default:
							if c, ok := BufferClients.Load(playlistID + stream.MD5); ok {

								var clients = c.(ClientConnection)
								if clients.Error != nil {
									ShowError(clients.Error, 0)
									killClientConnection(streamID, playlistID, false)
									return
								}

							} else {

								return

							}

						}

					}

					if _, err := bufferVFS.Stat(stream.Folder); fsIsNotExistErr(err) {
						killClientConnection(streamID, playlistID, false)
						return
					}

					var tmpFiles = getBufTmpFiles(&stream)

					for _, f := range tmpFiles {

						if _, err := bufferVFS.Stat(stream.Folder); fsIsNotExistErr(err) {
							killClientConnection(streamID, playlistID, false)
							return
						}

						var fileName = stream.Folder + f

						// Rotation of expired segments belongs to the producer
						// (thirdPartyBuffer), not here: with more than one
						// client on a stream, each client deleting from its own
						// private history means one client unlinks the segment
						// another is still reading.
						if err := sendSegmentToClient(stream.Folder, fileName, w, &debug); err != nil {
							killClientConnection(streamID, playlistID, false)
							return
						}

						segmentsSinceSync++

					}

					// Periodically sync OldSegments back to BufferInformation so
					// reconnecting clients have up-to-date segment history
					if segmentsSinceSync >= 5 {
						segmentsSinceSync = 0
						syncOldSegments(playlistID, streamID, stream.OldSegments)
					}

					if len(tmpFiles) == 0 {
						time.Sleep(time.Duration(25) * time.Millisecond)
					}

				} // End Loop 2

			} else {

				// Stream not found
				showDebug("Streaming Status:Stream not found. Killing Connection", 3)
				killClientConnection(streamID, stream.PlaylistID, false)
				showInfo(fmt.Sprintf("Streaming Status:Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))
				return

			}

		} // End BufferInformation

	} // End Loop 1

}

// sendSegmentToClient copies one buffer segment to the client.
//
// The read and the write are deliberately separated. The read happens under the
// stream guard so the segment cannot be unlinked underneath it (see
// buffer_guard.go); the write to the client happens after the guard is
// released, because a player that stops reading can block a write for a long
// time and must never be able to stall the teardown of a dead stream.
func sendSegmentToClient(folder, fileName string, w http.ResponseWriter, debug *string) error {
	var guard = streamGuardFor(folder)

	bufPtr := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufPtr)

	n, err := guard.readSegment(fileName, bufPtr)
	if err != nil {
		*debug = fmt.Sprintf("Buffer Open (%s)", fileName)
		showDebug(*debug, 2)
		return err
	}

	if n == 0 {
		return nil
	}

	*debug = fmt.Sprintf("Buffer Status:Send to client (%s)", fileName)
	showDebug(*debug, 2)

	if _, err := w.Write((*bufPtr)[:n]); err != nil {
		return err
	}

	// Flush data to client immediately to reduce streaming latency
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

func getBufTmpFiles(stream *ThisStream) (tmpFiles []string) {

	var tmpFolder = stream.Folder
	var fileIDs []float64

	if _, err := bufferVFS.Stat(tmpFolder); !fsIsNotExistErr(err) {

		files, err := bufferVFS.ReadDir(getPlatformPath(tmpFolder))
		if err != nil {
			ShowError(err, 000)
			return
		}

		if len(files) > 2 {

			for _, file := range files {

				var fileID = strings.Replace(file.Name(), ".ts", "", -1)
				var f, err = strconv.ParseFloat(fileID, 64)

				if err == nil {
					fileIDs = append(fileIDs, f)
				}

			}

			sort.Float64s(fileIDs)
			fileIDs = fileIDs[:len(fileIDs)-1]

			// When OldSegments is empty (new or reconnecting client) and there are
			// many segments on disk, skip to near the live edge. This prevents
			// replaying the entire buffer when a player reconnects.
			if len(stream.OldSegments) == 0 && len(fileIDs) > 2 {
				// Mark all but the last 2 segments as already seen
				for _, file := range fileIDs[:len(fileIDs)-2] {
					var fileName = fmt.Sprintf("%d.ts", int64(file))
					stream.OldSegments = append(stream.OldSegments, fileName)
				}
				fileIDs = fileIDs[len(fileIDs)-2:]
			}

			for _, file := range fileIDs {

				var fileName = fmt.Sprintf("%d.ts", int64(file))

				if indexOfString(fileName, stream.OldSegments) == -1 {
					tmpFiles = append(tmpFiles, fileName)
					stream.OldSegments = append(stream.OldSegments, fileName)
				}

			}

		}

		// Prevent unbounded growth of OldSegments for long-running streams
		if len(stream.OldSegments) > 100 {
			stream.OldSegments = stream.OldSegments[len(stream.OldSegments)-100:]
		}

	}

	return
}

func killClientConnection(streamID int, playlistID string, force bool) {
	Lock.Lock()
	defer Lock.Unlock()

	if p, ok := BufferInformation.Load(playlistID); ok {
		var playlist = p.(Playlist)

		if force {
			delete(playlist.Streams, streamID)
			if len(playlist.Streams) == 0 {
				BufferInformation.Delete(playlistID)
			} else {
				BufferInformation.Store(playlistID, playlist)
			}
			showInfo(fmt.Sprintf("Streaming Status: Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))
			return
		}

		if stream, ok := playlist.Streams[streamID]; ok {
			client := playlist.Clients[streamID]

			if c, ok := BufferClients.Load(playlistID + stream.MD5); ok {
				var clients = c.(ClientConnection)
				clients.Connection--
				client.Connection--

				// Ensure client connections cannot go below zero
				if client.Connection < 0 {
					client.Connection = 0
				}
				if clients.Connection < 0 {
					clients.Connection = 0
				}

				playlist.Clients[streamID] = client
				BufferClients.Store(playlistID+stream.MD5, clients)

				showInfo(fmt.Sprintf("Streaming Status: Channel: %s (Clients: %d)", stream.ChannelName, clients.Connection))

				if clients.Connection <= 0 {
					BufferClients.Delete(playlistID + stream.MD5)
					delete(playlist.Streams, streamID)
					delete(playlist.Clients, streamID)

					if len(playlist.Streams) == 0 {
						BufferInformation.Delete(playlistID)
					} else {
						BufferInformation.Store(playlistID, playlist)
					}
				} else {
					BufferInformation.Store(playlistID, playlist)
				}

				if len(playlist.Streams) > 0 {
					showInfo(fmt.Sprintf("Streaming Status: Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))
				}
			}
		}
	}
}

func clientConnection(stream ThisStream) (status bool) {

	status = true
	Lock.Lock()
	defer Lock.Unlock()

	if _, ok := BufferClients.Load(stream.PlaylistID + stream.MD5); !ok {

		var debug = fmt.Sprintf("Streaming Status:Remove temporary files (%s)", stream.Folder)
		showDebug(debug, 1)

		status = false

		debug = fmt.Sprintf("Remove tmp folder:%s", stream.Folder)
		showDebug(debug, 1)

		if err := closeStreamBuffer(stream.Folder); err != nil {
			ShowError(err, 4005)
		}

		if p, ok := BufferInformation.Load(stream.PlaylistID); !ok {

			showInfo(fmt.Sprintf("Streaming Status:Channel: %s - No client is using this channel anymore. Streaming Server connection has ended", stream.ChannelName))

			if p != nil {
				var playlist = p.(Playlist)

				showInfo(fmt.Sprintf("Streaming Status:Playlist: %s - Tuner: %d / %d", playlist.PlaylistName, len(playlist.Streams), playlist.Tuner))

				if len(playlist.Streams) <= 0 {
					BufferInformation.Delete(stream.PlaylistID)
				}
			}

		}

		status = false

	}

	return
}

func parseM3U8(stream *ThisStream) (err error) {

	var debug string
	var noNewSegment = false
	var lastSegmentDuration float64
	var segment Segment
	var m3u8Segments []Segment
	var sequence int64

	stream.DynamicBandwidth = false

	debug = fmt.Sprintf(`M3U8 Playlist:`+"\n"+`%s`, stream.Body)
	showDebug(debug, 3)

	var getBandwidth = func(line string) int {

		var infos = strings.Split(line, ",")

		for _, info := range infos {

			if strings.Contains(info, "BANDWIDTH=") {

				var bandwidth = strings.Replace(info, "BANDWIDTH=", "", -1)
				n, err := strconv.Atoi(bandwidth)
				if err == nil {
					return n
				}

			}

		}

		return 0
	}

	var parseParameter = func(line string, segment *Segment) (err error) {

		line = strings.Trim(line, "\r\n")

		var parameters = []string{"#EXT-X-VERSION:", "#EXT-X-PLAYLIST-TYPE:", "#EXT-X-MEDIA-SEQUENCE:", "#EXT-X-STREAM-INF:", "#EXTINF:"}

		for _, parameter := range parameters {

			if strings.Contains(line, parameter) {

				var value = strings.Replace(line, parameter, "", -1)

				switch parameter {

				case "#EXT-X-VERSION:":
					version, err := strconv.Atoi(value)
					if err == nil {
						segment.Version = version
					}

				case "#EXT-X-PLAYLIST-TYPE:":
					segment.PlaylistType = value

				case "#EXT-X-MEDIA-SEQUENCE:":
					n, err := strconv.ParseInt(value, 10, 64)
					if err == nil {
						stream.Sequence = n
						sequence = n
					}

				case "#EXT-X-STREAM-INF:":
					segment.Info = true
					segment.StreamInf.Bandwidth = getBandwidth(value)

				case "#EXTINF:":
					var d = strings.Split(value, ",")
					if len(d) > 0 {

						value = strings.Replace(d[0], ",", "", -1)
						duration, err := strconv.ParseFloat(value, 64)
						if err == nil {
							segment.Duration = duration
						} else {
							ShowError(err, 1050)
							return err
						}

					}

				}

			}

		}

		return
	}

	var parseURL = func(line string, segment *Segment) {

		// Check if the address is a valid URL (http://... or /path/to/stream)
		_, err := neturl.ParseRequestURI(line)
		if err == nil {

			// Check if the domain is included in the address
			u, _ := neturl.Parse(line)

			if len(u.Host) == 0 {
				// Address does not contain the domain, redirect is added to the address
				segment.URL = stream.URLStreamingServer + line
			} else {
				// Domain included in the address
				segment.URL = line
			}

		} else {

			// Not a URL, but a file path (media/file-01.ts)
			var serverURLPath = strings.Replace(stream.M3U8URL, path.Base(stream.M3U8URL), line, -1)
			segment.URL = serverURLPath

		}
	}

	if strings.Contains(stream.Body, "#EXTM3U") {

		var lines = strings.Split(strings.Replace(stream.Body, "\r\n", "\n", -1), "\n")

		if !stream.DynamicBandwidth {
			stream.DynamicStream = make(map[int]DynamicStream)
		}

		// Parse parameters
		for i, line := range lines {

			_ = i

			if len(line) > 0 {

				if line[0:1] == "#" {

					err := parseParameter(line, &segment)
					if err != nil {
						return err
					}

					lastSegmentDuration = segment.Duration

				}

				// M3U8 contains multiple links to other M3U8 playlists (bandwidth options)
				if segment.Info && len(line) > 0 && line[0:1] != "#" {

					var dynamicStream DynamicStream

					segment.Duration = 0
					noNewSegment = false

					stream.DynamicBandwidth = true
					parseURL(line, &segment)

					dynamicStream.Bandwidth = segment.StreamInf.Bandwidth
					dynamicStream.URL = segment.URL

					stream.DynamicStream[dynamicStream.Bandwidth] = dynamicStream

				}

				// Segment with TS stream
				if segment.Duration > 0 && line[0:1] != "#" {

					parseURL(line, &segment)

					if len(segment.URL) > 0 {
						segment.Sequence = sequence
						m3u8Segments = append(m3u8Segments, segment)
						sequence++
					}

				}

			}

		}

	} else {

		err = errors.New(getErrMsg(4051))
		return
	}

	if len(m3u8Segments) > 0 {

		noNewSegment = true

		if !stream.Status {

			if len(m3u8Segments) >= 2 {
				m3u8Segments = m3u8Segments[0 : len(m3u8Segments)-1]
			}

		}

		for _, s := range m3u8Segments {

			segment = s

			if !stream.Status {

				noNewSegment = false
				stream.LastSequence = segment.Sequence

				// Stream is of type VOD. The first segment of the M3U8 playlist must be used.
				if strings.ToUpper(segment.PlaylistType) == "VOD" {
					break
				}

			} else {

				if segment.Sequence > stream.LastSequence {

					stream.LastSequence = segment.Sequence
					noNewSegment = false
					break

				}

			}

		}

	}

	if !noNewSegment {

		if stream.DynamicBandwidth {
			switchBandwidth(stream)
		} else {
			stream.Segment = append(stream.Segment, segment)
		}

	}

	if noNewSegment {

		var sleep = lastSegmentDuration * 0.5

		for i := 0.0; i < sleep*1000; i = i + 100 {

			_ = i
			time.Sleep(time.Duration(100) * time.Millisecond)

			if _, err := bufferVFS.Stat(stream.Folder); fsIsNotExistErr(err) {
				break
			}

		}

	}

	return
}

func switchBandwidth(stream *ThisStream) (err error) {

	var bandwidth []int
	var dynamicStream DynamicStream
	var segment Segment

	for key := range stream.DynamicStream {
		bandwidth = append(bandwidth, key)
	}

	sort.Ints(bandwidth)

	if len(bandwidth) > 0 {

		for i := range bandwidth {

			segment.StreamInf.Bandwidth = stream.DynamicStream[bandwidth[i]].Bandwidth

			dynamicStream = stream.DynamicStream[bandwidth[0]]

			if stream.NetworkBandwidth == 0 {

				dynamicStream = stream.DynamicStream[bandwidth[0]]
				break

			} else {

				if bandwidth[i] > stream.NetworkBandwidth {
					break
				}

				dynamicStream = stream.DynamicStream[bandwidth[i]]

			}

		}

	} else {

		err = errors.New("M3U8 does not contain streaming URLs")
		return

	}

	segment.URL = dynamicStream.URL
	segment.Duration = 0
	stream.Segment = append(stream.Segment, segment)

	return
}

// Buffer with FFMPEG
func thirdPartyBuffer(streamID int, playlistID string, useBackup bool, backupNumber int) {

	if playlist, ok := getPlaylistSnapshot(playlistID); ok {

		var debug, path, options, bufferType string
		var tmpSegment = 1

		// Snapshot the settings this stream needs once. Settings is replaced
		// wholesale by saveSettings, so reading its fields repeatedly from a
		// long-lived goroutine races with every WebUI save.
		systemMutex.Lock()
		var bufferSize = Settings.BufferSize * 1024
		var ffmpegPath = Settings.FFmpegPath
		var ffmpegOptions = Settings.FFmpegOptions
		var vlcPath = Settings.VLCPath
		var vlcOptions = Settings.VLCOptions
		var userAgent = Settings.UserAgent
		var forceHTTP = Settings.FFmpegForceHttp
		systemMutex.Unlock()

		stream, streamFound := getStreamSnapshot(playlistID, streamID)
		if !streamFound {
			return
		}

		var buf bytes.Buffer
		var fileSize = 0

		var tmpFolder = stream.Folder
		var url = stream.URL
		if useBackup {
			if backupNumber >= 1 && backupNumber <= 3 {
				switch backupNumber {
				case 1:
					if stream.BackupChannel1 != nil {
						url = stream.BackupChannel1.URL
						showHighlight("START OF BACKUP 1 STREAM")
						showInfo("Backup Channel 1 URL: " + url)
					}
				case 2:
					if stream.BackupChannel2 != nil {
						url = stream.BackupChannel2.URL
						showHighlight("START OF BACKUP 2 STREAM")
						showInfo("Backup Channel 2 URL: " + url)
					}
				case 3:
					if stream.BackupChannel3 != nil {
						url = stream.BackupChannel3.URL
						showHighlight("START OF BACKUP 3 STREAM")
						showInfo("Backup Channel 3 URL: " + url)
					}
				}
			}
		}

		stream.Status = false

		var addErrorToStream = func(err error) {
			// Try backup channels if available and not yet exhausted
			if backupNumber < 3 && (stream.BackupChannel1 != nil || stream.BackupChannel2 != nil || stream.BackupChannel3 != nil) {
				backupNumber = backupNumber + 1
				thirdPartyBuffer(streamID, playlistID, true, backupNumber)
				return
			}

			// No backups available or all backups exhausted: propagate error to
			// BufferClients so the client's streaming loop (Loop 2) can detect
			// it and disconnect cleanly instead of hanging forever without data.
			stream, ok := getStreamSnapshot(playlistID, streamID)
			if !ok {
				return
			}

			if c, ok := BufferClients.Load(playlistID + stream.MD5); ok {

				var clients = c.(ClientConnection)
				clients.Error = err
				BufferClients.Store(playlistID+stream.MD5, clients)

			}

		}

		// Validate streaming URL scheme to prevent option injection
		parsedStreamURL, parseErr := neturl.Parse(url)
		if parseErr != nil || parsedStreamURL.Scheme == "" {
			log.Printf("Invalid streaming URL: %s", url)
			addErrorToStream(fmt.Errorf("invalid streaming URL: %s", url))
			return
		}
		allowedSchemes := map[string]bool{"http": true, "https": true, "rtsp": true, "rtp": true, "udp": true, "mmsh": true}
		if !allowedSchemes[strings.ToLower(parsedStreamURL.Scheme)] {
			log.Printf("Unsupported streaming URL scheme: %s", parsedStreamURL.Scheme)
			addErrorToStream(fmt.Errorf("unsupported streaming URL scheme: %s", parsedStreamURL.Scheme))
			return
		}
		url = parsedStreamURL.String()

		bufferType = strings.ToUpper(playlist.Buffer)

		switch playlist.Buffer {

		case "ffmpeg":

			if forceHTTP {
				url = strings.Replace(url, "https://", "http://", -1)
				showInfo("Forcing URL to HTTP for FFMPEG: " + url)
			}

			path = ffmpegPath
			options = ffmpegOptions

		case "vlc":
			path = vlcPath
			options = vlcOptions

		default:
			return
		}

		// Only wipe buffer folder on first invocation, not on backup channel switches.
		// This preserves existing segments so the client sees continuous playback.
		if !useBackup {
			if err := closeStreamBuffer(tmpFolder); err != nil {
				ShowError(err, 4005)
			}
		}

		err := checkVFSFolder(tmpFolder, bufferVFS)
		if err != nil {
			ShowError(err, 0)
			addErrorToStream(err)
			return
		}

		err = checkFile(path)
		if err != nil {
			ShowError(err, 0)
			addErrorToStream(err)
			return
		}

		showInfo(fmt.Sprintf("%s path:%s", bufferType, path))
		showInfo("Streaming URL:" + url)

		// Build command arguments once (reused across retries)
		var args []string

		for i, a := range strings.Split(options, " ") {

			switch bufferType {
			case "FFMPEG":
				a = strings.Replace(a, "[URL]", url, -1)
				if i == 0 {
					if len(userAgent) != 0 {
						args = []string{"-user_agent", userAgent}
					}

					if playlist.HttpProxyIP != "" && playlist.HttpProxyPort != "" {
						args = append(args, "-http_proxy", fmt.Sprintf("http://%s:%s", playlist.HttpProxyIP, playlist.HttpProxyPort))
					}

					var headers string
					if len(playlist.HttpUserReferer) != 0 {
						headers += fmt.Sprintf("Referer: %s\r\n", playlist.HttpUserReferer)
					}
					if len(playlist.HttpUserOrigin) != 0 {
						headers += fmt.Sprintf("Origin: %s\r\n", playlist.HttpUserOrigin)
					}
					if headers != "" {
						args = append(args, "-headers", headers)
					}
				}

				args = append(args, a)

			case "VLC":
				if a == "[URL]" {
					a = strings.Replace(a, "[URL]", url, -1)
					args = append(args, a)

					if len(userAgent) != 0 {
						args = append(args, fmt.Sprintf(":http-user-agent=%s", userAgent))
					}

					if len(playlist.HttpUserReferer) != 0 {
						args = append(args, fmt.Sprintf(":http-referrer=%s", playlist.HttpUserReferer))
					}

					if playlist.HttpProxyIP != "" && playlist.HttpProxyPort != "" {
						args = append(args, fmt.Sprintf(":http-proxy=%s:%s", playlist.HttpProxyIP, playlist.HttpProxyPort))
					}

				} else {
					args = append(args, a)
				}

			}

		}

		// Pre-flight: ask the upstream hop whether the channel is still there
		// before spending 20 seconds of the viewer's time finding out through
		// FFmpeg. A rotated-away channel id answers 404 immediately, and there
		// is no point retrying that one.
		if probe := probeStreamOrigin(url, userAgent); !probe.Reachable {
			showInfo(fmt.Sprintf("Streaming Status:Upstream did not answer for %s (%s)", stream.ChannelName, probe))
		} else if probe.Gone {
			var goneErr = fmt.Errorf("channel %s is no longer published upstream (HTTP %d)", stream.ChannelName, probe.StatusCode)
			ShowError(goneErr, 4008)
			addErrorToStream(goneErr)

			return
		}

		// Retry loop: when FFmpeg/VLC exits on a stream that was working,
		// restart on the same URL before falling through to backup channels.
		const maxSameURLRetries = 3
		const retryDelaySec = 2
		var retryCount int

		for retryCount <= maxSameURLRetries {

			var tmpFile = fmt.Sprintf("%s%d.ts", tmpFolder, tmpSegment)

			f, err := bufferVFS.Create(tmpFile)
			if err != nil {
				ShowError(err, 0)
				addErrorToStream(err)
				return
			}
			f.Close()

			var cmd = exec.Command(path, args...)
			cmd.Env = append(os.Environ(), "DISPLAY=:0")

			debug = fmt.Sprintf("BUFFER DEBUG: %s:%s %s", bufferType, path, args)
			showDebug(debug, 1)

			stdOut, err := cmd.StdoutPipe()
			if err != nil {
				ShowError(err, 0)
				addErrorToStream(err)
				return
			}

			logOut, err := cmd.StderrPipe()
			if err != nil {
				ShowError(err, 0)
				addErrorToStream(err)
				return
			}

			if len(buf.Bytes()) == 0 && !stream.Status {
				showInfo(bufferType + ":Processing data")
			}

			// Fresh channel per iteration to avoid double-close panic
			var streamStatus = make(chan bool)
			var streamStatusClosed bool

			cmd.Start()

			go func() {
				scanner := bufio.NewScanner(logOut)
				scanner.Split(bufio.ScanLines)
				for scanner.Scan() {
					// Local, not the enclosing `debug`: that one is also
					// written by the read loop, and sharing it was a data race
					// for no benefit.
					var line = fmt.Sprintf("%s log:%s", bufferType, strings.TrimSpace(scanner.Text()))
					select {
					case <-streamStatus:
						showDebug(line, 1)
					default:
						showInfo(line)
					}
					time.Sleep(time.Duration(10) * time.Millisecond)
				}
			}()

			f, err = bufferVFS.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				ShowError(err, 0)
				cmd.Process.Kill()
				cmd.Wait()
				addErrorToStream(err)
				return
			}

			buffer := make([]byte, 128*1024) // 128KB buffer for better throughput
			reader := bufio.NewReader(stdOut)

			// Inactivity watchdog.
			//
			// The old timeout only armed while tmpSegment == 1, so it stopped
			// existing the moment the first segment was written. After that a
			// backend that accepted the connection and went quiet - what
			// happens when the provider blocks the VPN exit node - left
			// reader.Read blocked forever: no data, no EOF, no client check,
			// the FFmpeg process still alive and the tuner still occupied until
			// Threadfin was restarted.
			//
			// The watchdog now runs for the whole life of the stream and kills
			// the process when nothing has arrived for the configured window,
			// which unblocks the read and lets the normal error path run.
			watchdogCtx, stopWatchdog := context.WithCancel(context.Background())

			var lastData atomic.Int64
			lastData.Store(time.Now().UnixNano())

			var inactive atomic.Bool

			// Non-EOF read error, reported after the loop.
			var readErr error

			go func() {
				var window = inactivityTimeout()
				if window <= 0 {
					return
				}

				var ticker = time.NewTicker(time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-watchdogCtx.Done():
						return

					case <-ticker.C:
						if time.Since(time.Unix(0, lastData.Load())) < window {
							continue
						}

						inactive.Store(true)
						showInfo(fmt.Sprintf("Streaming Status:No data from %s for %s, ending stream", bufferType, window))

						if cmd.Process != nil {
							cmd.Process.Kill()
						}

						return
					}
				}
			}()

			// Inner read loop: read data from FFmpeg/VLC stdout and write to segment files
			for {

				if fileSize == 0 && !stream.Status {
					showInfo("Streaming Status:Receive data from " + bufferType)
				}

				if !clientConnection(stream) {
					cmd.Process.Kill()
					f.Close()
					cmd.Wait()
					stopWatchdog()
					return
				}

				n, err := reader.Read(buffer)
				if err != nil {
					// Only io.EOF used to end the loop. Any other error left
					// n == 0 and the loop spinning without sleeping or making
					// progress, which pins a core on a machine that has four.
					if !errors.Is(err, io.EOF) {
						showDebug(fmt.Sprintf("%s read error: %s", bufferType, err), 1)
						readErr = err
					}

					break
				}

				if n > 0 {
					lastData.Store(time.Now().UnixNano())
				}

				fileSize = fileSize + len(buffer[:n])

				if _, err := f.Write(buffer[:n]); err != nil {
					cmd.Process.Kill()
					ShowError(err, 0)
					addErrorToStream(err)
					cmd.Wait()
					stopWatchdog()
					f.Close()
					return
				}

				if fileSize >= bufferSize/2 {

					if tmpSegment == 1 && !stream.Status && !streamStatusClosed {
						// Only quietens the stderr log from here on. The
						// watchdog has its own lifetime and must keep running:
						// cancelling it here is what left a stalled stream
						// hanging forever once the first segment was written.
						close(streamStatus)
						streamStatusClosed = true
						showInfo(fmt.Sprintf("Streaming Status:Buffering data from %s", bufferType))
					}

					f.Close()
					tmpSegment++

					// Retire segments that have fallen out of the buffer
					// window. This used to be done by each client on its own
					// private history, which meant one client could unlink a
					// segment another client was still reading.
					if expired := tmpSegment - bufferedSegments; expired > 0 {
						if err := removeStreamSegment(tmpFolder, fmt.Sprintf("%d.ts", expired)); err != nil {
							ShowError(err, 4007)
						}
					}

					if !stream.Status {
						stream.Status = true
						setStreamStatus(playlistID, streamID, true)
					}

					tmpFile = fmt.Sprintf("%s%d.ts", tmpFolder, tmpSegment)

					fileSize = 0

					// Close the handle Create returns; it used to be
					// discarded, which is harmless on memfs but a descriptor
					// leak on any other filesystem.
					created, errCreate := bufferVFS.Create(tmpFile)
					if errCreate == nil {
						created.Close()
					}

					var errOpen error
					f, errOpen = bufferVFS.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0600)
					if errCreate != nil || errOpen != nil {
						// Report the error that actually happened: this used to
						// pass the enclosing err, which is usually nil here.
						var segmentErr = errCreate
						if segmentErr == nil {
							segmentErr = errOpen
						}

						cmd.Process.Kill()
						ShowError(segmentErr, 0)
						addErrorToStream(segmentErr)
						cmd.Wait()
						stopWatchdog()
						return
					}

				}

			} // End inner read loop

			// FFmpeg/VLC exited (EOF), either on its own or because the
			// watchdog killed it. Clean up this iteration.
			cmd.Process.Kill()
			cmd.Wait()
			stopWatchdog()
			f.Close()

			if inactive.Load() {
				// EC 4006. Report it before deciding on a retry so a client
				// waiting in Loop 1 stops waiting.
				var timeoutErr = fmt.Errorf("no data from %s for %s", bufferType, inactivityTimeout())
				ShowError(timeoutErr, 4006)
				addErrorToStream(timeoutErr)

				return
			}

			if readErr != nil {
				ShowError(readErr, 1204)
				addErrorToStream(readErr)

				return
			}

			// Retry decision: only retry if stream was producing data (tmpSegment > 1).
			// If it failed before the first segment, the stream is likely invalid.
			if tmpSegment > 1 && retryCount < maxSameURLRetries {
				retryCount++

				// Exponential backoff. A fixed two seconds meant three rapid
				// restarts against a URL the provider had already rotated
				// away, which is three FFmpeg launches and six seconds of
				// nothing for the viewer.
				var delay = time.Duration(retryDelaySec<<(retryCount-1)) * time.Second

				showInfo(fmt.Sprintf("Streaming Status:%s exited after %d segments, retry %d/%d in %s...",
					bufferType, tmpSegment-1, retryCount, maxSameURLRetries, delay))

				if !clientConnection(stream) {
					return
				}

				// Sleep in short slices so a client leaving during the backoff
				// is noticed promptly instead of after the full delay.
				for waited := time.Duration(0); waited < delay; waited += 250 * time.Millisecond {
					time.Sleep(250 * time.Millisecond)

					if !clientConnection(stream) {
						return
					}
				}

				if !clientConnection(stream) {
					return
				}

				// Reset fileSize for new segment but keep tmpSegment for continuous numbering
				fileSize = 0
				continue
			}

			// Stream never produced data or retries exhausted
			break

		} // End retry loop

		err = errors.New(bufferType + " error")
		ShowError(err, 1204)
		addErrorToStream(err)

		time.Sleep(time.Duration(500) * time.Millisecond)
		clientConnection(stream)

		return

	}

}

func getTuner(id, playlistType string) (tuner int) {

	var playListBuffer string
	systemMutex.Lock()
	playListInterface := Settings.Files.M3U[id]
	if playListInterface == nil {
		playListInterface = Settings.Files.HDHR[id]
	}
	if playListMap, ok := playListInterface.(map[string]interface{}); ok {
		if buffer, ok := playListMap["buffer"].(string); ok {
			playListBuffer = buffer
		} else {
			playListBuffer = "-"
		}
	}
	systemMutex.Unlock()

	switch playListBuffer {

	case "-":
		tuner = Settings.Tuner

	case "threadfin", "ffmpeg", "vlc":

		i, err := strconv.Atoi(getProviderParameter(id, playlistType, "tuner"))
		if err == nil {
			tuner = i
		} else {
			ShowError(err, 0)
			tuner = 1
		}

	}

	return
}

// initBufferVFS creates the buffer filesystem. It is called once at startup and
// must not be called again while streams are running: replacing the global
// would strand every open handle and every live stream folder.
//
// The buffer is always in RAM. The storeBufferInRAM setting is inert - upstream
// had an osfs branch, this fork does not - which is worth knowing before
// reaching for that switch to reduce SD card wear: it is already not touching
// the card.
func initBufferVFS() {
	bufferVFS = memfs.New()
}

func debugRequest(req *http.Request) {

	var debugLevel = 3

	if System.Flag.Debug < debugLevel {
		return
	}

	var debug string

	fmt.Println()
	debug = "Request:* * * * * * BEGIN HTTP(S) REQUEST * * * * * * "
	showDebug(debug, debugLevel)

	debug = fmt.Sprintf("Method:%s", req.Method)
	showDebug(debug, debugLevel)

	debug = fmt.Sprintf("Proto:%s", req.Proto)
	showDebug(debug, debugLevel)

	debug = fmt.Sprintf("URL:%s", req.URL)
	showDebug(debug, debugLevel)

	for name, headers := range req.Header {

		name = strings.ToLower(name)

		for _, h := range headers {
			debug = fmt.Sprintf("Header:%v: %v", name, h)
			showDebug(debug, debugLevel)
		}

	}

	debug = "Request:* * * * * * END HTTP(S) REQUEST * * * * * *"
	showDebug(debug, debugLevel)

	return
}

func debugResponse(resp *http.Response) {

	var debugLevel = 3

	if System.Flag.Debug < debugLevel {
		return
	}

	var debug string

	fmt.Println()

	debug = "Response:* * * * * * BEGIN RESPONSE * * * * * * "
	showDebug(debug, debugLevel)

	debug = fmt.Sprintf("Proto:%s", resp.Proto)
	showDebug(debug, debugLevel)

	debug = fmt.Sprintf("Status Code:%d", resp.StatusCode)
	showDebug(debug, debugLevel)

	debug = fmt.Sprintf("Status Text:%s", http.StatusText(resp.StatusCode))
	showDebug(debug, debugLevel)

	for key, value := range resp.Header {

		switch fmt.Sprintf("%T", value) {

		case "[]string":
			debug = fmt.Sprintf("Header:%v: %s", key, strings.Join(value, " "))

		default:
			debug = fmt.Sprintf("Header:%v: %v", key, value)
		}

		showDebug(debug, debugLevel)

	}

	debug = "Pesponse:* * * * * * END RESPONSE * * * * * * "
	showDebug(debug, debugLevel)

	return
}

func terminateProcessGracefully(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// Send a SIGTERM to the process
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			// If an error occurred while trying to send the SIGTERM, you might resort to a SIGKILL.
			ShowError(err, 0)
			cmd.Process.Kill()
		}

		// Optionally, you can wait for the process to finish too
		cmd.Wait()
	}
}
