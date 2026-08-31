package config

import "testing"

func TestLoadKnownHosts_MissingFile_ReturnsEmptyMap(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	hosts, err := LoadKnownHosts()
	if err != nil {
		t.Fatalf("LoadKnownHosts() unexpected error: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected empty map for a missing known_hosts.yaml, got %v", hosts)
	}
}

func TestSaveAndLoadKnownHosts_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	want := map[string]KnownHostKey{
		"10.0.1.5:22":           {KeyType: "ssh-ed25519", Fingerprint: "SHA256:abc123"},
		"host.example.com:2222": {KeyType: "ssh-rsa", Fingerprint: "SHA256:def456"},
	}
	if err := SaveKnownHosts(want); err != nil {
		t.Fatalf("SaveKnownHosts() unexpected error: %v", err)
	}

	got, err := LoadKnownHosts()
	if err != nil {
		t.Fatalf("LoadKnownHosts() unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d: %v", len(want), len(got), got)
	}
	for key, wantKey := range want {
		gotKey, ok := got[key]
		if !ok {
			t.Errorf("missing entry for %q", key)
			continue
		}
		if gotKey != wantKey {
			t.Errorf("entry for %q = %+v, want %+v", key, gotKey, wantKey)
		}
	}
}

func TestSaveKnownHosts_SecondSaveReplacesFirst(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	if err := SaveKnownHosts(map[string]KnownHostKey{
		"a:22": {KeyType: "ssh-rsa", Fingerprint: "SHA256:first"},
	}); err != nil {
		t.Fatalf("first SaveKnownHosts() unexpected error: %v", err)
	}
	if err := SaveKnownHosts(map[string]KnownHostKey{
		"b:22": {KeyType: "ssh-rsa", Fingerprint: "SHA256:second"},
	}); err != nil {
		t.Fatalf("second SaveKnownHosts() unexpected error: %v", err)
	}

	got, err := LoadKnownHosts()
	if err != nil {
		t.Fatalf("LoadKnownHosts() unexpected error: %v", err)
	}
	if _, ok := got["a:22"]; ok {
		t.Error("expected the first save's entry to be gone after the second (full-replace) save")
	}
	if _, ok := got["b:22"]; !ok {
		t.Error("expected the second save's entry to be present")
	}
}
