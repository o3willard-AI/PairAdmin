package services

import "strings"

// This file holds the platform-neutral half of the "Launch PairAdmin as Local
// Windows Administrator" feature: the startup decision, the token-elevation
// model, and the command-line quoting used to relaunch.
//
// It is deliberately untagged so it builds and, more importantly, is TESTED on
// every platform — CI's Linux runner included. The Windows-only pieces (reading
// the process token, ShellExecuteEx) live in elevation_windows.go behind the
// function values elevationPlan takes as arguments.

// ElevationOutcome is what startup decided to do about the administrator
// toggle. Returning it (rather than only acting on it) is what lets a test
// assert on the decision without a process exit in the middle of the run.
type ElevationOutcome int

const (
	// ElevationNotRequested: the setting is off (its default). Nothing happens.
	ElevationNotRequested ElevationOutcome = iota
	// ElevationAlreadyElevated: the setting is on and this process already holds
	// a full administrator token — i.e. this IS the relaunched instance. It must
	// keep running and must not relaunch again.
	ElevationAlreadyElevated
	// ElevationRelaunching: the setting is on, this process is not elevated, and
	// an elevated copy has been started. This process exits.
	ElevationRelaunching
	// ElevationProbeFailed: the token could not be read. Treated as "do not
	// relaunch" — see elevationPlan.
	ElevationProbeFailed
	// ElevationRelaunchFailed: the elevated copy could not be started (most
	// often the user declined the UAC prompt). This process keeps running.
	ElevationRelaunchFailed
)

// elevationPlan is the whole policy of the feature, and the only place that
// decides whether to relaunch.
//
// Why a child process at all: a Windows child inherits its parent's token, so
// if PairAdmin runs elevated then every ConPTY shell it spawns (cmd.exe,
// powershell.exe, …) is elevated too — with no per-terminal UAC prompt, and
// with the terminal still embedded where the assistant can read it. That makes
// the feature exactly one toggle plus one startup relaunch, and this function
// is the relaunch.
//
// probe, relaunch and exit are passed in rather than called directly so the
// decision is testable on every platform; the Windows caller supplies
// isElevated, relaunchAsAdmin and os.Exit.
func elevationPlan(
	launchAsAdministrator bool,
	probe func() (bool, error),
	relaunch func() error,
	exit func(int),
) ElevationOutcome {
	if !launchAsAdministrator {
		return ElevationNotRequested
	}

	elevated, err := probe()
	if err != nil {
		// Fail closed. A token we could not read is not evidence that we need
		// elevation, and guessing "not elevated" would put a UAC prompt in
		// front of a user who never asked for one.
		return ElevationProbeFailed
	}
	if elevated {
		// This is the instance the relaunch produced. Relaunching again here
		// would spawn a fresh elevated process on every startup — an unbounded
		// UAC loop, not a slow start.
		return ElevationAlreadyElevated
	}

	if err := relaunch(); err != nil {
		// UAC declined, or ShellExecuteEx refused. Exit here and the user
		// double-clicks the icon and nothing happens; keep running and they get
		// a working (unelevated) PairAdmin plus, on Windows, a message.
		return ElevationRelaunchFailed
	}

	// The elevated copy owns the window now.
	exit(0)
	return ElevationRelaunching
}

// TokenElevation is how the current process token relates to the administrator
// token. Modelled here (rather than as a raw windows.TokenElevationType) so the
// mapping can be tested on any platform; elevation_windows.go maps the real
// syscall value onto it.
type TokenElevation int

const (
	// TokenElevationUnknown: the token type was not one we recognise.
	TokenElevationUnknown TokenElevation = iota
	// TokenElevationDefault: UAC never split the token (UAC disabled, or a
	// pre-UAC account). Not an administrator token.
	TokenElevationDefault
	// TokenElevationFull: the real administrator token.
	TokenElevationFull
	// TokenElevationLimited: an administrator running with the filtered token.
	// Still needs a "run as administrator" relaunch — this is the case a
	// username or Administrators-group check gets wrong.
	TokenElevationLimited
)

// IsFull reports whether this elevation type means the process actually holds
// the administrator token.
//
// The accuracy bar for this feature is explicit: elevation is decided by the
// token's ELEVATION TYPE, never by a username or an Administrators-group
// membership check. Only Full counts — a Limited token belongs to an
// administrator whose token is filtered, and treating that as "already
// elevated" would silently skip the relaunch the user asked for.
func (e TokenElevation) IsFull() bool {
	return e == TokenElevationFull
}

// buildArgString renders args as one Windows command line, using the same
// quoting the C runtime uses to split it back apart (Microsoft, "Parsing C++
// Command-Line Arguments"; the algorithm subprocess.list2cmdline implements):
//
//   - an argument with no space, tab or quote, and not empty, passes through;
//   - otherwise it is wrapped in double quotes;
//   - a run of backslashes immediately before a quote (including the closing
//     quote) is doubled, so the backslashes survive and the quote still
//     escapes.
//
// ShellExecuteEx takes the parameters as a single string, so the relaunch has
// to re-quote argv itself; getting this wrong turns
// `--config "C:\Program Files\PairAdmin\config.yaml"` into two arguments.
func buildArgString(args []string) string {
	var b strings.Builder
	for i, arg := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(quoteWindowsArg(arg))
	}
	return b.String()
}

func quoteWindowsArg(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n\v\"") {
		return arg
	}

	var b strings.Builder
	b.WriteByte('"')
	backslashes := 0
	for _, r := range arg {
		switch r {
		case '\\':
			backslashes++
		case '"':
			b.WriteString(strings.Repeat(`\`, backslashes*2+1))
			b.WriteByte('"')
			backslashes = 0
		default:
			b.WriteString(strings.Repeat(`\`, backslashes))
			backslashes = 0
			b.WriteRune(r)
		}
	}
	// Trailing backslashes would otherwise escape the closing quote.
	b.WriteString(strings.Repeat(`\`, backslashes*2))
	b.WriteByte('"')
	return b.String()
}
