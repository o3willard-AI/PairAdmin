package keychain

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/scrypt"

	"pairadmin/services/config"
)

// The file backend of 99designs/keyring encrypts every item with AES-256-GCM
// under a password supplied by Config.FilePasswordFunc. PairAdmin previously
// hardcoded that password (keyring.FixedStringPrompt("pairadmin")), so any
// copy of the binary could decrypt the stored items — the encryption was
// effectively obfuscation.
//
// Instead, the Client now holds a user-chosen master password in memory and
// persists only a salted scrypt verification hash in
// ~/.pairadmin/master_password.hash (mode 0600), in the self-describing format
//
//	scrypt$N$r$p$<salt-hex>$<hash-hex>
//
// so future parameter changes remain verifiable against old hash files.

const (
	// masterPasswordHashFile is the name of the hash file inside ~/.pairadmin.
	masterPasswordHashFile = "master_password.hash"

	// Parameters used for NEW hashes written by SetMasterPassword and
	// ChangeMasterPassword. Verification always uses the parameters stored in
	// the hash file itself.
	//
	// N=32768, r=8, p=1 costs ~32 MiB of scrypt memory and ~50-100 ms per
	// hash: negligible for a human-driven desktop app, meaningful for brute
	// force.
	scryptN       = 1 << 15
	scryptR       = 8
	scryptP       = 1
	scryptKeyLen  = 32
	scryptSaltLen = 16

	// Acceptance bounds when parsing an existing hash file. Files written by
	// PairAdmin always fall inside these bounds; the bounds exist to refuse a
	// corrupted or hostile hash file with absurd parameters (e.g. N large
	// enough to exhaust memory during verification).
	minScryptN = 1 << 14
	maxScryptN = 1 << 20
	maxScryptR = 64
	maxScryptP = 8
)

// Sentinel errors for master password handling, exported so callers (e.g.
// future Wails bindings) can map them to user-facing messages.
var (
	// ErrNoMasterPassword is returned when the file backend asks for the
	// keyring password but no verified master password is held in memory.
	ErrNoMasterPassword = errors.New("master password required: call SetMasterPassword or VerifyMasterPassword first")

	// ErrWrongMasterPassword is returned by ChangeMasterPassword when the old
	// password does not match the stored hash.
	ErrWrongMasterPassword = errors.New("incorrect master password")

	// ErrMasterPasswordExists is returned by SetMasterPassword when a master
	// password is already configured.
	ErrMasterPasswordExists = errors.New("master password already set: use ChangeMasterPassword to change it")
)

// masterPasswordHashPath returns the path of the master password hash file,
// ConfigDir()/master_password.hash — the same per-user data dir as config.yaml,
// known_hosts.yaml, and the keyring file backend (see config.ConfigDir).
func masterPasswordHashPath() (string, error) {
	return filepath.Join(config.ConfigDir(), masterPasswordHashFile), nil
}

// writeMasterPasswordHash writes a salted scrypt hash of pw to path in the
// self-describing format "scrypt$N$r$p$<salt-hex>$<hash-hex>". The file is
// written to a temporary file in the same directory and renamed into place,
// so an interrupted write can never leave a truncated (unusable) hash file.
func writeMasterPasswordHash(path, pw string) error {
	salt := make([]byte, scryptSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}
	hash, err := scrypt.Key([]byte(pw), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return fmt.Errorf("derive scrypt hash: %w", err)
	}
	line := fmt.Sprintf("scrypt$%d$%d$%d$%s$%s",
		scryptN, scryptR, scryptP,
		hex.EncodeToString(salt), hex.EncodeToString(hash))

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+masterPasswordHashFile+"-*")
	if err != nil {
		return fmt.Errorf("create temporary hash file: %w", err)
	}
	tmpName := tmp.Name()
	// No-op once the rename below succeeds.
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(line); err != nil {
		tmp.Close()
		return fmt.Errorf("write hash: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("restrict hash file permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync hash file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close hash file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install hash file: %w", err)
	}
	return nil
}

// parseMasterPasswordHash parses the self-describing hash format
// "scrypt$N$r$p$<salt-hex>$<hash-hex>" and validates that the stored
// parameters are within accepted bounds.
func parseMasterPasswordHash(line string) (n, r, p int, salt, hash []byte, err error) {
	parts := strings.Split(line, "$")
	if len(parts) != 6 {
		return 0, 0, 0, nil, nil, fmt.Errorf("malformed master password hash: expected 6 '$'-separated fields, got %d", len(parts))
	}
	if parts[0] != "scrypt" {
		return 0, 0, 0, nil, nil, fmt.Errorf("unsupported hash algorithm %q (want \"scrypt\")", parts[0])
	}
	if n, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("parse scrypt N: %w", err)
	}
	if r, err = strconv.Atoi(parts[2]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("parse scrypt r: %w", err)
	}
	if p, err = strconv.Atoi(parts[3]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("parse scrypt p: %w", err)
	}
	if n < minScryptN || n > maxScryptN || r < 1 || r > maxScryptR || p < 1 || p > maxScryptP {
		return 0, 0, 0, nil, nil, fmt.Errorf("unsupported scrypt parameters N=%d r=%d p=%d", n, r, p)
	}
	if salt, err = hex.DecodeString(parts[4]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("parse salt: %w", err)
	}
	if len(salt) == 0 {
		return 0, 0, 0, nil, nil, errors.New("empty salt in master password hash")
	}
	if hash, err = hex.DecodeString(parts[5]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("parse hash: %w", err)
	}
	if len(hash) != scryptKeyLen {
		return 0, 0, 0, nil, nil, fmt.Errorf("unexpected hash length %d (want %d)", len(hash), scryptKeyLen)
	}
	return n, r, p, salt, hash, nil
}

// verifyMasterPasswordHash reports whether pw matches the hash stored at
// path. A missing hash file yields (false, nil); a present but malformed
// hash file is an error — verification fails closed rather than silently
// rejecting the password.
func verifyMasterPasswordHash(path, pw string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read master password hash: %w", err)
	}
	n, r, p, salt, want, err := parseMasterPasswordHash(strings.TrimSpace(string(data)))
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	got, err := scrypt.Key([]byte(pw), salt, n, r, p, len(want))
	if err != nil {
		return false, fmt.Errorf("derive scrypt hash: %w", err)
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
