package services

import (
	"testing"

	"pairadmin/services/config"
)

func TestRemoteService_SaveRemoteHost_GeneratesIDAndStoresSecret(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	host := config.RemoteHost{Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu", AuthType: "password"}
	saved, err := svc.SaveRemoteHost(host, "hunter2", "")
	if err != nil {
		t.Fatalf("SaveRemoteHost() unexpected error: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("expected generated ID, got empty string")
	}
	if saved.LastUsed == "" {
		t.Error("expected LastUsed to be set")
	}

	pw, err := kc.Get(remoteKeychainKey(saved.ID, "password"))
	if err != nil {
		t.Fatalf("keychain Get() unexpected error: %v", err)
	}
	if pw != "hunter2" {
		t.Errorf("expected stored password %q, got %q", "hunter2", pw)
	}
}

func TestRemoteService_SaveRemoteHost_NoSecretWhenPasswordEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	host := config.RemoteHost{Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu", AuthType: "privatekey", PrivateKeyPath: "/home/user/.ssh/id_ed25519"}
	saved, err := svc.SaveRemoteHost(host, "", "")
	if err != nil {
		t.Fatalf("SaveRemoteHost() unexpected error: %v", err)
	}

	if pw, _ := kc.Get(remoteKeychainKey(saved.ID, "password")); pw != "" {
		t.Errorf("expected no password stored, got %q", pw)
	}
}

func TestRemoteService_SaveRemoteHost_UpsertByID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	first, err := svc.SaveRemoteHost(config.RemoteHost{Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu"}, "pw1", "")
	if err != nil {
		t.Fatalf("first SaveRemoteHost() unexpected error: %v", err)
	}

	// Re-save with the same ID but a different host — should update, not duplicate.
	first.Host = "10.0.1.99"
	_, err = svc.SaveRemoteHost(first, "", "")
	if err != nil {
		t.Fatalf("second SaveRemoteHost() unexpected error: %v", err)
	}

	hosts, err := svc.ListRemoteHosts()
	if err != nil {
		t.Fatalf("ListRemoteHosts() unexpected error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host after upsert, got %d", len(hosts))
	}
	if hosts[0].Host != "10.0.1.99" {
		t.Errorf("expected updated host '10.0.1.99', got %q", hosts[0].Host)
	}
}

func TestRemoteService_ListRemoteHosts_MostRecentFirst(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	if err := config.SaveAppConfig(&config.AppConfig{
		RemoteHosts: []config.RemoteHost{
			{ID: "old", Host: "old-host", LastUsed: "2026-01-01T00:00:00Z"},
			{ID: "new", Host: "new-host", LastUsed: "2026-06-01T00:00:00Z"},
		},
	}); err != nil {
		t.Fatalf("seed SaveAppConfig() unexpected error: %v", err)
	}

	hosts, err := svc.ListRemoteHosts()
	if err != nil {
		t.Fatalf("ListRemoteHosts() unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].ID != "new" {
		t.Errorf("expected most-recently-used host first, got %q", hosts[0].ID)
	}
}

func TestRemoteService_ForgetRemoteHost_RemovesConfigAndSecret(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	saved, err := svc.SaveRemoteHost(config.RemoteHost{Kind: "ssh", Host: "10.0.1.5"}, "hunter2", "")
	if err != nil {
		t.Fatalf("SaveRemoteHost() unexpected error: %v", err)
	}

	if err := svc.ForgetRemoteHost(saved.ID); err != nil {
		t.Fatalf("ForgetRemoteHost() unexpected error: %v", err)
	}

	hosts, err := svc.ListRemoteHosts()
	if err != nil {
		t.Fatalf("ListRemoteHosts() unexpected error: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts after forget, got %d", len(hosts))
	}
	if pw, _ := kc.Get(remoteKeychainKey(saved.ID, "password")); pw != "" {
		t.Errorf("expected password removed from keychain, got %q", pw)
	}
}

func TestRemoteService_TouchRemoteHost_UpdatesLastUsed(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	// Seed directly (bypassing SaveRemoteHost, which always stamps "now") so
	// oldLastUsed is a distant, unambiguously-stale value to compare against.
	const oldLastUsed = "2020-01-01T00:00:00Z"
	if err := config.SaveAppConfig(&config.AppConfig{
		RemoteHosts: []config.RemoteHost{{ID: "host-1", Host: "10.0.1.5", LastUsed: oldLastUsed}},
	}); err != nil {
		t.Fatalf("seed SaveAppConfig() unexpected error: %v", err)
	}

	if err := svc.TouchRemoteHost("host-1"); err != nil {
		t.Fatalf("TouchRemoteHost() unexpected error: %v", err)
	}

	hosts, err := svc.ListRemoteHosts()
	if err != nil {
		t.Fatalf("ListRemoteHosts() unexpected error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].LastUsed == oldLastUsed {
		t.Error("expected LastUsed to be updated by TouchRemoteHost")
	}
}

func TestRemoteService_TouchRemoteHost_UnknownID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	if err := svc.TouchRemoteHost("does-not-exist"); err == nil {
		t.Error("expected error for unknown host ID, got nil")
	}
}

func TestRemoteService_RenameRemoteHost_SetsNameWithoutClobberingOtherFields(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	saved, err := svc.SaveRemoteHost(config.RemoteHost{
		Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu", AuthType: "password", UseTmux: true, TmuxSessionName: "work",
	}, "hunter2", "")
	if err != nil {
		t.Fatalf("SaveRemoteHost() unexpected error: %v", err)
	}

	if err := svc.RenameRemoteHost(saved.ID, "Prod Web Server"); err != nil {
		t.Fatalf("RenameRemoteHost() unexpected error: %v", err)
	}

	hosts, err := svc.ListRemoteHosts()
	if err != nil {
		t.Fatalf("ListRemoteHosts() unexpected error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	h := hosts[0]
	if h.Name != "Prod Web Server" {
		t.Errorf("Name: expected 'Prod Web Server', got %q", h.Name)
	}
	// Every other field must survive the rename untouched.
	if h.Host != "10.0.1.5" || h.Username != "ubuntu" || h.Port != 22 || !h.UseTmux || h.TmuxSessionName != "work" {
		t.Errorf("expected other fields to survive RenameRemoteHost untouched, got %+v", h)
	}

	// The password must still be retrievable — renaming must not touch the keychain.
	pw, err := kc.Get(remoteKeychainKey(saved.ID, "password"))
	if err != nil {
		t.Fatalf("keychain Get() unexpected error: %v", err)
	}
	if pw != "hunter2" {
		t.Errorf("expected password to survive rename, got %q", pw)
	}
}

func TestRemoteService_RenameRemoteHost_UnknownID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	kc := makeTestKeychainClient(mem)
	svc := NewRemoteService(kc)

	if err := svc.RenameRemoteHost("does-not-exist", "New Name"); err == nil {
		t.Error("expected error for unknown host ID, got nil")
	}
}
