package keychain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/99designs/keyring"

	"pairadmin/services/config"
)

// This file pins three security properties of the master-password handling
// that a mutation audit found untested (K2/K3/K4). Each test is written so
// that removing the corresponding security behavior in the production code
// turns it red:
//
//   - K2: verifyMasterPasswordHash must treat a MISSING hash file as
//     "not configured" — (false, nil) — not an error (the startup flow
//     branches on this to show the first-run dialog).
//   - K3: parseMasterPasswordHash must reject scrypt parameters outside the
//     acceptance bounds (N outside 2^14..2^20, r > 64, p > 8) so a hostile
//     or corrupted hash file cannot make verification allocate absurd memory
//     or run degenerate work factors.
//   - K4: SetMasterPassword must reject an empty password — an empty
//     passphrase would make the file backend trivially decryptable.

// K2 — missing hash file → (false, nil).
//
// Mutation check: deleting the `os.IsNotExist(err) { return false, nil }`
// branch in verifyMasterPasswordHash makes this test fail (the call returns
// a read error instead of a clean false).
func TestVerifyMasterPassword_MissingHashFileReturnsFalseNil(t *testing.T) {
	home := isolateHome(t)
	// No ~/.pairadmin/master_password.hash is written — the directory may not
	// even exist yet, which is exactly the fresh-install state.
	c := NewWithOpenFunc(func(cfg keyring.Config) (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	})

	ok, err := c.VerifyMasterPassword("whatever-password")
	if err != nil {
		t.Fatalf("VerifyMasterPassword with no hash file: expected (false, nil), got error: %v", err)
	}
	if ok {
		t.Error("VerifyMasterPassword with no hash file: expected ok=false, got true")
	}

	// And the hash file really is absent — guard against the test silently
	// passing because something created it.
	if _, err := os.Stat(filepath.Join(home, ".pairadmin", masterPasswordHashFile)); !os.IsNotExist(err) {
		t.Fatalf("expected no hash file to exist, got stat error: %v", err)
	}
}

// K3 — hostile scrypt parameters are rejected.
//
// Mutation check: removing the bounds check in parseMasterPasswordHash (the
// `n < minScryptN || n > maxScryptN || r < 1 || r > maxScryptR || p < 1 ||
// p > maxScryptP` condition) makes every case in this test fail.
func TestParseMasterPasswordHash_RejectsHostileScryptParams(t *testing.T) {
	// salt: 16 bytes, hash: 32 bytes — well-formed below the params.
	const validHexTail = "aabbccddeeff00112233445566778899" + // salt (16 bytes)
		"$" +
		"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff" // hash (32 bytes)

	cases := []struct {
		name string
		line string
	}{
		{"N below minimum", "scrypt$1$8$1$" + validHexTail},
		{"N just below minimum", "scrypt$16383$8$1$" + validHexTail},
		{"N above maximum (memory exhaustion)", "scrypt$1073741824$8$1$" + validHexTail}, // 1<<30
		{"N just above maximum", "scrypt$1048577$8$1$" + validHexTail},                   // 1<<20 + 1
		{"r above maximum", "scrypt$32768$65$1$" + validHexTail},
		{"r below minimum", "scrypt$32768$0$1$" + validHexTail},
		{"p above maximum", "scrypt$32768$8$9$" + validHexTail},
		{"p below minimum", "scrypt$32768$8$0$" + validHexTail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, _, _, err := parseMasterPasswordHash(tc.line)
			if err == nil {
				t.Errorf("parseMasterPasswordHash(%q): expected hostile params to be rejected, got nil error", tc.line)
				return
			}
			if !strings.Contains(err.Error(), "unsupported scrypt parameters") {
				t.Errorf("parseMasterPasswordHash(%q): expected 'unsupported scrypt parameters' error, got: %v", tc.line, err)
			}
		})
	}

	// Control: the shipped parameter set (N=2^15, r=8, p=1) parses fine — the
	// bounds reject hostility, not legitimate files.
	t.Run("legitimate shipped params still accepted", func(t *testing.T) {
		line := "scrypt$32768$8$1$" + validHexTail
		if _, _, _, _, _, err := parseMasterPasswordHash(line); err != nil {
			t.Errorf("parseMasterPasswordHash(%q): expected the shipped parameter set to parse, got: %v", line, err)
		}
	})

	// Control: boundary values are inclusive — the exact min N and max N/r/p
	// must parse, so the bounds are neither off-by-one strict nor loose.
	t.Run("boundary params accepted inclusively", func(t *testing.T) {
		for _, line := range []string{
			"scrypt$16384$8$1$" + validHexTail,   // minScryptN exactly
			"scrypt$1048576$8$1$" + validHexTail, // maxScryptN exactly
			"scrypt$32768$64$1$" + validHexTail,  // maxScryptR exactly
			"scrypt$32768$8$8$" + validHexTail,   // maxScryptP exactly
		} {
			if _, _, _, _, _, err := parseMasterPasswordHash(line); err != nil {
				t.Errorf("parseMasterPasswordHash(%q): expected inclusive boundary to parse, got: %v", line, err)
			}
		}
	})
}

// K4 — empty master password is rejected by SetMasterPassword.
//
// Mutation check: removing the `pw == ""` guard in SetMasterPassword makes
// this test fail (the empty password is written as a hash and accepted).
func TestSetMasterPassword_RejectsEmptyPassword(t *testing.T) {
	isolateHome(t)
	c := newFileBackendTestClient()

	err := c.SetMasterPassword("")
	if err == nil {
		t.Fatal("SetMasterPassword(\"\"): expected an error, got nil")
	}
	if errors.Is(err, ErrMasterPasswordExists) {
		t.Errorf("SetMasterPassword(\"\"): got ErrMasterPasswordExists, want an empty-password rejection")
	}

	// The empty password must not have been persisted in any form: no hash
	// file may exist after the rejection, so the next startup still lands in
	// the first-run flow (NeedsMasterPassword-style probe). Note the
	// in-memory hold can't be asserted via Get() here: the file backend only
	// invokes FilePasswordFunc when an item file exists, and none does — a
	// Get on an empty keyring returns ErrKeyNotFound (mapped to "", nil)
	// without ever consulting the password.
	if _, err := os.Stat(filepath.Join(config.ConfigDir(), masterPasswordHashFile)); !os.IsNotExist(err) {
		t.Error("SetMasterPassword(\"\"): hash file was created despite the rejection")
	}
}
