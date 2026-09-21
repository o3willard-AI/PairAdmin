package scanner

import (
	"testing"

	"pairadmin/services/config"
)

// TestScanAllowed_HonorsScannerEnabled is the anti-gaming (mutation-style)
// proof that the seam links the config flag to the allow decision.
//
// Mutation check: deleting the `return c.ScannerEnabled` body of scanAllowed
// (or replacing the field reference with a hardcoded `return true`) turns
// this test red — the false case would report allowed=true for a
// disabled scanner, exactly the security hole this seam closes. Deleting the
// ScannerEnabled field or its mapstructure/yaml tag likewise makes the false
// case fail to persist (see the config round-trip test for the load path).
func TestScanAllowed_HonorsScannerEnabled(t *testing.T) {
	if allowed := scanAllowed(config.AppConfig{ScannerEnabled: false}); allowed {
		t.Fatal("expected scanAllowed to be false when ScannerEnabled is false")
	}
	if allowed := scanAllowed(config.AppConfig{ScannerEnabled: true}); !allowed {
		t.Fatal("expected scanAllowed to be true when ScannerEnabled is true")
	}
}