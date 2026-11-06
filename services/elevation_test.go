package services

import (
	"errors"
	"testing"
)

// These tests cover the platform-neutral half of the "Launch as administrator"
// feature: the startup decision table (elevationPlan), the token-elevation
// mapping, and the Windows command-line quoting. They live in an untagged file
// so they run on every platform, including CI's Linux runner — the Windows
// probe and the relaunch call are injected as function values, which is exactly
// the seam that makes that possible.

func TestElevationPlan_ToggleOffNeverTouchesTheProcess(t *testing.T) {
	probes, relaunches, exits := 0, 0, 0

	outcome := elevationPlan(false,
		func() (bool, error) { probes++; return false, nil },
		func() error { relaunches++; return nil },
		func(int) { exits++ },
	)

	if outcome != ElevationNotRequested {
		t.Errorf("outcome: expected ElevationNotRequested, got %v", outcome)
	}
	// Mutation check: dropping the `if !launchAsAdministrator` early return in
	// elevationPlan makes the process probe its token and relaunch even with
	// the setting off (the default) — every counter below becomes 1 and this
	// fails.
	if probes != 0 || relaunches != 0 || exits != 0 {
		t.Errorf("toggle off must be inert: probes=%d relaunches=%d exits=%d", probes, relaunches, exits)
	}
}

func TestElevationPlan_RelaunchesWhenEnabledAndNotElevated(t *testing.T) {
	relaunches, exits := 0, -1

	outcome := elevationPlan(true,
		func() (bool, error) { return false, nil },
		func() error { relaunches++; return nil },
		func(code int) { exits = code },
	)

	if outcome != ElevationRelaunching {
		t.Errorf("outcome: expected ElevationRelaunching, got %v", outcome)
	}
	// Counterweight to the "already elevated" case below: the non-elevated path
	// must actually relaunch, or the feature does nothing.
	// Mutation check: removing the relaunch() call (or the os.Exit after it)
	// makes relaunches/exits 0 here — the elevated instance never starts, or
	// the unelevated one never gets out of the way.
	if relaunches != 1 {
		t.Errorf("expected exactly 1 relaunch, got %d", relaunches)
	}
	if exits != 0 {
		t.Errorf("expected exit(0) after a successful relaunch, got exit(%d)", exits)
	}
}

func TestElevationPlan_AlreadyElevatedDoesNotLoop(t *testing.T) {
	relaunches, exits := 0, 0

	outcome := elevationPlan(true,
		func() (bool, error) { return true, nil },
		func() error { relaunches++; return nil },
		func(int) { exits++ },
	)

	if outcome != ElevationAlreadyElevated {
		t.Errorf("outcome: expected ElevationAlreadyElevated, got %v", outcome)
	}
	// This is the instance the relaunch produced. If it relaunched again the app
	// would spawn a fresh elevated process on every startup, each one prompting
	// for UAC — an unbounded loop, not a slow start.
	// Mutation check: deleting the `if elevated { return ElevationAlreadyElevated }`
	// branch in elevationPlan makes relaunches and exits 1, failing both
	// assertions below.
	if relaunches != 0 {
		t.Errorf("an already-elevated process must not relaunch again, got %d relaunches", relaunches)
	}
	if exits != 0 {
		t.Errorf("an already-elevated process must keep running, got %d exits", exits)
	}
}

func TestElevationPlan_ProbeErrorDoesNotRelaunch(t *testing.T) {
	relaunches, exits := 0, 0

	outcome := elevationPlan(true,
		func() (bool, error) { return false, errors.New("token read failed") },
		func() error { relaunches++; return nil },
		func(int) { exits++ },
	)

	if outcome != ElevationProbeFailed {
		t.Errorf("outcome: expected ElevationProbeFailed, got %v", outcome)
	}
	// Fail closed: an unreadable token is not evidence that we need elevation.
	// Prompting for UAC because a syscall failed would train the user to accept
	// a prompt they did not ask for.
	// Mutation check: treating the error as "not elevated" (the common
	// `elevated, _ := probe()` slip) makes relaunches 1 and this fails.
	if relaunches != 0 || exits != 0 {
		t.Errorf("probe failure must be inert: relaunches=%d exits=%d", relaunches, exits)
	}
}

func TestElevationPlan_RelaunchFailureKeepsTheAppRunning(t *testing.T) {
	exits := 0

	outcome := elevationPlan(true,
		func() (bool, error) { return false, nil },
		func() error { return errors.New("UAC declined") },
		func(int) { exits++ },
	)

	if outcome != ElevationRelaunchFailed {
		t.Errorf("outcome: expected ElevationRelaunchFailed, got %v", outcome)
	}
	// A declined UAC prompt must leave a usable app, not a dead one: if this
	// exits without a relaunched instance, the user double-clicks the icon and
	// nothing happens.
	// Mutation check: moving os.Exit(0) above the relaunch error check makes
	// exits 1 here and the app vanishes when the user declines the prompt.
	if exits != 0 {
		t.Errorf("a failed relaunch must not exit the process, got %d exits", exits)
	}
}

func TestTokenElevation_OnlyFullIsElevated(t *testing.T) {
	cases := []struct {
		name       string
		elevation  TokenElevation
		wantIsFull bool
	}{
		// The accuracy bar for this feature: elevation is decided by the token's
		// elevation TYPE. TokenElevationFull is the administrator token;
		// TokenElevationLimited is the same administrator running with the
		// filtered token, which still needs a "run as administrator" relaunch;
		// TokenElevationDefault means UAC never split the token at all.
		{"full", TokenElevationFull, true},
		{"limited admin token is not elevated", TokenElevationLimited, false},
		{"default (no UAC split) is not elevated", TokenElevationDefault, false},
		{"unknown is not elevated", TokenElevationUnknown, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.elevation.IsFull(); got != tc.wantIsFull {
				t.Errorf("TokenElevation(%d).IsFull(): expected %v, got %v", tc.elevation, tc.wantIsFull, got)
			}
		})
	}
	// Mutation check: widening IsFull to `e != TokenElevationUnknown` (or to a
	// `!= Default` check) flips the limited-admin case to true — the app would
	// then decide an unelevated admin is already elevated, and never relaunch.
}

func TestBuildArgString_PreservesArgsForTheRelaunch(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no args", nil, ""},
		{"single bare arg", []string{"--uninstall-cleanup"}, "--uninstall-cleanup"},
		{"space is quoted", []string{"--config", `C:\Program Files\PairAdmin\config.yaml`},
			`--config "C:\Program Files\PairAdmin\config.yaml"`},
		{"embedded quote is escaped", []string{`say "hi"`}, `"say \"hi\""`},
		{"trailing backslash in a quoted arg is doubled", []string{`C:\Program Files\`}, `"C:\Program Files\\"`},
		{"bare trailing backslash needs no quoting", []string{`C:\dir\`}, `C:\dir\`},
		{"empty arg is preserved as an empty quoted token", []string{""}, `""`},
		{"order and count are preserved", []string{"a", "b c", "d"}, `a "b c" d`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildArgString(tc.args); got != tc.want {
				t.Errorf("buildArgString(%q): expected %q, got %q", tc.args, tc.want, got)
			}
		})
	}
	// Mutation check: replacing buildArgString with strings.Join(args, " ")
	// drops the quoting, so the space/quote/empty-arg cases come back
	// unquoted and fail — the elevated instance would then parse a path like
	// C:\Program Files\... as two arguments.
}
