package src

import (
	"strings"
	"testing"
)

// The options string actually running in production (from threadfinxpanic.log,
// minus the -user_agent pair that buffer.go prepends at run time). It is
// customised - c:a copy rather than aac, plus max_muxing_queue_size - so it
// matches none of the shipped defaults, which is exactly the case that has to
// be migrated rather than replaced.
const productionFFmpegOptions = "-hide_banner -loglevel error -reconnect 1 -reconnect_at_eof 1 " +
	"-reconnect_streamed 1 -reconnect_on_network_error 1 -reconnect_delay_max 10 -i [URL] " +
	"-c:v copy -c:a copy -f mpegts -max_muxing_queue_size 512 pipe:1"

func TestMigrationAddsReadTimeoutToCustomisedOptions(t *testing.T) {
	settings := SettingsStruct{FFmpegOptions: productionFFmpegOptions}

	if !migrateSettings(&settings) {
		t.Fatal("migration reported no change")
	}

	if !hasFFmpegReadTimeout(settings.FFmpegOptions) {
		t.Fatalf("no read timeout was added: %s", settings.FFmpegOptions)
	}

	// The user's own choices must survive.
	for _, keep := range []string{"-c:a copy", "-max_muxing_queue_size 512", "-reconnect_delay_max 10"} {
		if !strings.Contains(settings.FFmpegOptions, keep) {
			t.Errorf("migration dropped %q: %s", keep, settings.FFmpegOptions)
		}
	}

	// -rw_timeout is an input option: FFmpeg ignores it after -i.
	var fields = strings.Fields(settings.FFmpegOptions)
	var timeoutAt, inputAt = -1, -1

	for i, f := range fields {
		if f == "-rw_timeout" && timeoutAt < 0 {
			timeoutAt = i
		}
		if f == "-i" && inputAt < 0 {
			inputAt = i
		}
	}

	if timeoutAt < 0 || inputAt < 0 || timeoutAt > inputAt {
		t.Fatalf("-rw_timeout must precede -i: %s", settings.FFmpegOptions)
	}

	if settings.SchemaVersion != currentSettingsSchema {
		t.Fatalf("schema version = %d, want %d", settings.SchemaVersion, currentSettingsSchema)
	}
}

func TestMigrationIsIdempotent(t *testing.T) {
	settings := SettingsStruct{FFmpegOptions: productionFFmpegOptions}

	migrateSettings(&settings)
	once := settings.FFmpegOptions

	if migrateSettings(&settings) {
		t.Fatal("a settings struct already at the current schema was migrated again")
	}

	if settings.FFmpegOptions != once {
		t.Fatalf("second migration changed the options:\n%s\n%s", once, settings.FFmpegOptions)
	}

	if strings.Count(settings.FFmpegOptions, "-rw_timeout") != 1 {
		t.Fatalf("duplicated -rw_timeout: %s", settings.FFmpegOptions)
	}
}

func TestMigrationReplacesUntouchedDefaults(t *testing.T) {
	restore := System.FFmpeg.DefaultOptions
	t.Cleanup(func() { System.FFmpeg.DefaultOptions = restore })

	System.FFmpeg.DefaultOptions = "-hide_banner -rw_timeout 15000000 -i [URL] -f mpegts pipe:1"

	for _, old := range knownFFmpegDefaults {
		settings := SettingsStruct{FFmpegOptions: old}

		if !migrateSettings(&settings) {
			t.Fatalf("a shipped default was not migrated: %s", old)
		}

		if settings.FFmpegOptions != System.FFmpeg.DefaultOptions {
			t.Fatalf("shipped default was not replaced wholesale:\n%s", settings.FFmpegOptions)
		}
	}
}

func TestMigrationLeavesExistingTimeoutAlone(t *testing.T) {
	for _, existing := range []string{
		"-hide_banner -rw_timeout 9000000 -i [URL] pipe:1",
		"-hide_banner -timeout 9000000 -i [URL] pipe:1",
		"-hide_banner -stimeout 9000000 -i [URL] pipe:1",
	} {
		settings := SettingsStruct{FFmpegOptions: existing}
		migrateSettings(&settings)

		if settings.FFmpegOptions != existing {
			t.Fatalf("migration overrode an existing timeout:\nwant %s\ngot  %s", existing, settings.FFmpegOptions)
		}
	}
}

func TestMigrationSkipsEmptyOptions(t *testing.T) {
	// An empty value is filled from the current default elsewhere; the
	// migration must not invent one.
	settings := SettingsStruct{}
	migrateSettings(&settings)

	if settings.FFmpegOptions != "" {
		t.Fatalf("migration wrote options into an empty field: %s", settings.FFmpegOptions)
	}
}

func TestMigrationHandlesOptionsWithoutInputFlag(t *testing.T) {
	const weird = "-hide_banner -loglevel error pipe:1"

	settings := SettingsStruct{FFmpegOptions: weird}
	migrateSettings(&settings)

	if settings.FFmpegOptions != weird {
		t.Fatalf("migration mangled options with no -i flag: %s", settings.FFmpegOptions)
	}
}
