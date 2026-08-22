package src

import (
	"fmt"
	"strings"
)

/*
Settings migrations.

Defaults in loadSettings only apply to keys that are absent from settings.json,
so improving a default has no effect on an existing install. That is how
64b8025's better FFmpeg options - the ones carrying -rw_timeout - never reached
any running deployment: every one of them already had ffmpeg.options saved.

settings.schema.version records which migrations have run. Installs that predate
it report 0 and get every migration; the version is bumped and saved once they
have run, so a migration never fights a deliberate later edit by the user.
*/

// currentSettingsSchema is the schema version this build expects.
const currentSettingsSchema = 1

// ffmpegReadTimeoutMicroseconds is the -rw_timeout value injected by migration
// 1: 15 seconds, in the microseconds FFmpeg expects. Long enough to ride out a
// slow provider, short enough that a mute backend fails fast.
const ffmpegReadTimeoutMicroseconds = 15000000

// knownFFmpegDefaults are the ffmpeg.options strings this project has shipped as
// its default. An install still carrying one of these has never been customised,
// so it can be replaced with the current default wholesale.
var knownFFmpegDefaults = []string{
	// 35d857b (upstream, ~1.2.32)
	"-hide_banner -loglevel error -analyzeduration 1000000 -probesize 1000000 -i [URL] -map 0:v -map 0:a:0 -c:v copy -c:a aac -b:a 192k -ac 2 -c:s copy -f mpegts -fflags +genpts -movflags +faststart -copyts pipe:1",
	// 579f29c
	"-hide_banner -loglevel error -analyzeduration 1000000 -probesize 1000000 -i [URL] -map 0:v -map 0:a:0 -c:v copy -c:a aac -b:a 192k -ac 2 -c:s copy -f mpegts -fflags +genpts+discardcorrupt pipe:1",
	// f704b17
	"-hide_banner -loglevel error -analyzeduration 1000000 -probesize 1000000 -i [URL] -map 0:v -map 0:a:0 -c:v copy -c:a aac -b:a 192k -ac 2 -c:s copy -f mpegts -mpegts_flags +resend_headers -fflags +genpts+discardcorrupt pipe:1",
	// e6b09b1
	"-hide_banner -loglevel error -reconnect 1 -reconnect_at_eof 1 -reconnect_streamed 1 -reconnect_delay_max 5 -analyzeduration 1000000 -probesize 1000000 -i [URL] -map 0:v -map 0:a:0 -c:v copy -c:a aac -b:a 192k -ac 2 -c:s copy -f mpegts -mpegts_flags +resend_headers -fflags +genpts+discardcorrupt pipe:1",
}

// hasFFmpegReadTimeout reports whether an option string already bounds how long
// FFmpeg will wait on a silent socket.
func hasFFmpegReadTimeout(options string) bool {
	for _, flag := range []string{"-rw_timeout", "-timeout", "-stimeout"} {
		if strings.Contains(options, flag+" ") {
			return true
		}
	}

	return false
}

// addFFmpegReadTimeout inserts -rw_timeout ahead of the input flag, preserving
// everything the user has customised. It returns the options unchanged if there
// is no input flag to anchor to, or if a timeout is already present.
func addFFmpegReadTimeout(options string, microseconds int) (string, bool) {
	if hasFFmpegReadTimeout(options) {
		return options, false
	}

	var fields = strings.Fields(options)
	var timeout = []string{"-rw_timeout", fmt.Sprint(microseconds)}

	for i, field := range fields {
		if field != "-i" {
			continue
		}

		// -rw_timeout is an input option: it has to precede -i.
		var merged = make([]string, 0, len(fields)+2)
		merged = append(merged, fields[:i]...)
		merged = append(merged, timeout...)
		merged = append(merged, fields[i:]...)

		return strings.Join(merged, " "), true
	}

	return options, false
}

// migrateSettings brings a settings struct loaded from disk up to the current
// schema. It returns true when something changed and the caller should save.
func migrateSettings(settings *SettingsStruct) (changed bool) {
	if settings.SchemaVersion >= currentSettingsSchema {
		return false
	}

	if settings.SchemaVersion < 1 {
		changed = migrateFFmpegReadTimeout(settings) || changed
	}

	settings.SchemaVersion = currentSettingsSchema

	return true
}

// migrateFFmpegReadTimeout is migration 1.
//
// Without a read timeout FFmpeg waits forever on a backend that accepted the
// connection and then went silent - which is exactly what happens when the
// provider blocks the VPN exit node. The reconnect flags make it worse: FFmpeg
// retries internally and never returns, so Threadfin never learns the stream is
// dead and the tuner stays occupied.
func migrateFFmpegReadTimeout(settings *SettingsStruct) (changed bool) {
	if len(settings.FFmpegOptions) == 0 {
		return false
	}

	for _, known := range knownFFmpegDefaults {
		if settings.FFmpegOptions != known {
			continue
		}

		settings.FFmpegOptions = System.FFmpeg.DefaultOptions
		showInfo("Settings:FFmpeg options were still at a previous default, updated to the current one")

		return true
	}

	updated, added := addFFmpegReadTimeout(settings.FFmpegOptions, ffmpegReadTimeoutMicroseconds)
	if !added {
		if !hasFFmpegReadTimeout(settings.FFmpegOptions) {
			showWarning(2097)
		}

		return false
	}

	settings.FFmpegOptions = updated
	showInfo(fmt.Sprintf("Settings:Added -rw_timeout %d to your customised FFmpeg options so a silent backend fails fast instead of hanging",
		ffmpegReadTimeoutMicroseconds))

	return true
}
