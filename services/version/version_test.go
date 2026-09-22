package version

import "testing"

func TestGetVersionDefaultIsDev(t *testing.T) {
	// The -ldflags release override (-X pairadmin/services/version.appVersion=<tag>)
	// cannot be exercised in a unit test — this pins the DEFAULT and the
	// non-emptiness contract the About tab relies on; the override itself is
	// exercised by every release build (release.yml) and its verify-version gate.
	//
	// Mutation check: changing the default to "" (or dropping the appVersion
	// var) fails this — the About tab must never render an empty version.
	if appVersion != "dev" {
		t.Errorf("default appVersion must be %q, got %q", "dev", appVersion)
	}
	if GetVersion() == "" {
		t.Error("GetVersion() must never return an empty string")
	}
	if GetVersion() != appVersion {
		t.Errorf("GetVersion() = %q, want the appVersion var %q", GetVersion(), appVersion)
	}
}
