package services

import (
	"testing"

	"github.com/masterzen/winrm"

	"pairadmin/services/config"
)

// capturedWinRMEndpoint records the *winrm.Endpoint the production code built
// for the most recent openWinRMTerminal call, so tests can assert TLS /
// insecure flags without a live WinRM server.
var capturedEndpoint *winrm.Endpoint

func withEndpointCapturingFactory(t *testing.T) {
	t.Helper()
	orig := winrmClientFactory
	t.Cleanup(func() { winrmClientFactory = orig })
	winrmClientFactory = func(endpoint *winrm.Endpoint, user, password string) (*winrm.Client, error) {
		capturedEndpoint = endpoint
		// Succeed the factory but let shell creation fail naturally against
		// an unreachable host — the endpoint is already captured by then.
		return orig(endpoint, user, password)
	}
}

// TestOpenWinRMTerminal_UseTLSEnablesTLSOnEndpoint verifies params.UseTLS
// drives winrm.NewEndpoint's HTTPS flag (the TLS path) and
// params.InsecureSkipVerify flows into its Insecure flag — the R-03 finding
// (WinRM had no TLS path at all).
func TestOpenWinRMTerminal_UseTLSEnablesTLSOnEndpoint(t *testing.T) {
	withEndpointCapturingFactory(t)
	svc, _ := newTestPTYService()

	_, _ = svc.openWinRMTerminal("winrm:tls-tab", RemoteConnectParams{
		Kind:               RemoteKindWinRM,
		Host:               "192.0.2.1", // TEST-NET-1, unreachable — fine, endpoint is captured before dialing
		Port:               5986,
		Username:           "Administrator",
		AuthType:           RemoteAuthPassword,
		Password:           "irrelevant",
		UseTLS:             true,
		InsecureSkipVerify: true,
	})

	if capturedEndpoint == nil {
		t.Fatal("expected winrmClientFactory to receive an endpoint")
	}
	if !capturedEndpoint.HTTPS {
		t.Error("expected Endpoint.HTTPS=true when params.UseTLS=true")
	}
	if !capturedEndpoint.Insecure {
		t.Error("expected Endpoint.Insecure=true when params.InsecureSkipVerify=true")
	}
}

// TestOpenWinRMTerminal_TLSWithoutSkipVerify verifies the security-relevant
// combination: TLS on with certificate verification still enabled.
func TestOpenWinRMTerminal_TLSWithoutSkipVerify(t *testing.T) {
	withEndpointCapturingFactory(t)
	svc, _ := newTestPTYService()

	_, _ = svc.openWinRMTerminal("winrm:tls-verify-tab", RemoteConnectParams{
		Kind:               RemoteKindWinRM,
		Host:               "192.0.2.1",
		Port:               5986,
		Username:           "Administrator",
		AuthType:           RemoteAuthPassword,
		Password:           "irrelevant",
		UseTLS:             true,
		InsecureSkipVerify: false,
	})

	if capturedEndpoint == nil {
		t.Fatal("expected endpoint capture")
	}
	if !capturedEndpoint.HTTPS {
		t.Error("expected Endpoint.HTTPS=true when params.UseTLS=true")
	}
	if capturedEndpoint.Insecure {
		t.Error("expected Endpoint.Insecure=false when params.InsecureSkipVerify=false (cert verification on)")
	}
}

// TestOpenWinRMTerminal_PlaintextNoRegression pins the pre-change behavior:
// UseTLS=false + InsecureSkipVerify=false must still produce the exact
// plaintext endpoint that shipped before TLS support (HTTPS=false,
// Insecure=false). Existing saved hosts (Go zero-value false) hit this path.
func TestOpenWinRMTerminal_PlaintextNoRegression(t *testing.T) {
	withEndpointCapturingFactory(t)
	svc, _ := newTestPTYService()

	_, _ = svc.openWinRMTerminal("winrm:plain-tab", RemoteConnectParams{
		Kind:     RemoteKindWinRM,
		Host:     "192.0.2.1",
		Port:     5985,
		Username: "Administrator",
		AuthType: RemoteAuthPassword,
		Password: "irrelevant",
		// UseTLS / InsecureSkipVerify deliberately unset (zero-value false),
		// exactly what an existing saved host round-trips as.
	})

	if capturedEndpoint == nil {
		t.Fatal("expected endpoint capture")
	}
	if capturedEndpoint.HTTPS {
		t.Error("expected Endpoint.HTTPS=false for plaintext opt-out, got TLS")
	}
	if capturedEndpoint.Insecure {
		t.Error("expected Endpoint.Insecure=false for plaintext opt-out")
	}
	if capturedEndpoint.Host != "192.0.2.1" {
		t.Errorf("expected endpoint host '192.0.2.1', got %q", capturedEndpoint.Host)
	}
	if capturedEndpoint.Port != 5985 {
		t.Errorf("expected explicit port 5985 preserved, got %d", capturedEndpoint.Port)
	}
}

// TestOpenWinRMTerminal_PortDefaults pins the port defaulting: an unset port
// (0) defaults to 5986 when UseTLS is on and 5985 for plaintext — a TLS
// connection must never silently fall back to the plaintext port.
func TestOpenWinRMTerminal_PortDefaults(t *testing.T) {
	withEndpointCapturingFactory(t)
	svc, _ := newTestPTYService()

	// TLS + unset port -> 5986
	capturedEndpoint = nil
	_, _ = svc.openWinRMTerminal("winrm:tls-port", RemoteConnectParams{
		Kind:     RemoteKindWinRM,
		Host:     "192.0.2.1",
		Port:     0,
		Username: "Administrator",
		AuthType: RemoteAuthPassword,
		Password: "irrelevant",
		UseTLS:   true,
	})
	if capturedEndpoint == nil || capturedEndpoint.Port != 5986 {
		t.Errorf("expected default port 5986 with UseTLS=true and unset port, got %+v", capturedEndpoint)
	}

	// Plaintext + unset port -> 5985 (unchanged behavior)
	capturedEndpoint = nil
	_, _ = svc.openWinRMTerminal("winrm:plain-port", RemoteConnectParams{
		Kind:     RemoteKindWinRM,
		Host:     "192.0.2.1",
		Port:     0,
		Username: "Administrator",
		AuthType: RemoteAuthPassword,
		Password: "irrelevant",
	})
	if capturedEndpoint == nil || capturedEndpoint.Port != 5985 {
		t.Errorf("expected default port 5985 with UseTLS=false and unset port, got %+v", capturedEndpoint)
	}

	// Explicit ports always win over defaults.
	capturedEndpoint = nil
	_, _ = svc.openWinRMTerminal("winrm:tls-custom-port", RemoteConnectParams{
		Kind:     RemoteKindWinRM,
		Host:     "192.0.2.1",
		Port:     15986,
		Username: "Administrator",
		AuthType: RemoteAuthPassword,
		Password: "irrelevant",
		UseTLS:   true,
	})
	if capturedEndpoint == nil || capturedEndpoint.Port != 15986 {
		t.Errorf("expected explicit custom port preserved, got %+v", capturedEndpoint)
	}
}

// TestResolveRemoteCredentials_AppliesSavedHostTLS verifies the saved-host
// threading: when a reconnect carries only SavedHostId, the persisted
// RemoteHost's UseTLS/InsecureSkipVerify are applied onto the connect params
// (and the port default follows), so a saved TLS host reconnects over TLS.
func TestResolveRemoteCredentials_AppliesSavedHostTLS(t *testing.T) {
	isolateHomeDir(t)

	// Persist a WinRM host with TLS enabled.
	if err := config.SaveAppConfig(&config.AppConfig{
		RemoteHosts: []config.RemoteHost{
			{
				ID:                 "winrm-saved-1",
				Kind:               "winrm",
				Host:               "192.0.2.1",
				Port:               5986,
				Username:           "Administrator",
				AuthType:           "password",
				UseTLS:             true,
				InsecureSkipVerify: true,
			},
		},
	}); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	svc, _ := newTestPTYService()
	svc.SetKeychainClient(makeTestKeychainClient(newInMemoryKeyring()))

	resolved, err := svc.resolveRemoteCredentials(RemoteConnectParams{
		Kind:        RemoteKindWinRM,
		Host:        "192.0.2.1",
		Username:    "Administrator",
		AuthType:    RemoteAuthPassword,
		SavedHostId: "winrm-saved-1",
		// Port/UseTLS deliberately unset — must come from the saved record.
	})
	if err != nil {
		t.Fatalf("resolveRemoteCredentials: %v", err)
	}
	if !resolved.UseTLS {
		t.Error("expected UseTLS=true applied from the saved host record")
	}
	if !resolved.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true applied from the saved host record")
	}
	if resolved.Port != 5986 {
		t.Errorf("expected saved port 5986 applied, got %d", resolved.Port)
	}

	// A saved PLAINTEXT host must stay plaintext.
	if err := config.SaveAppConfig(&config.AppConfig{
		RemoteHosts: []config.RemoteHost{
			{ID: "winrm-saved-2", Kind: "winrm", Host: "192.0.2.2", Port: 5985, Username: "Administrator", AuthType: "password"},
		},
	}); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}
	resolved2, err := svc.resolveRemoteCredentials(RemoteConnectParams{
		Kind:        RemoteKindWinRM,
		Host:        "192.0.2.2",
		Username:    "Administrator",
		AuthType:    RemoteAuthPassword,
		SavedHostId: "winrm-saved-2",
	})
	if err != nil {
		t.Fatalf("resolveRemoteCredentials (plaintext): %v", err)
	}
	if resolved2.UseTLS || resolved2.InsecureSkipVerify {
		t.Errorf("expected no TLS flags for a legacy plaintext saved host, got UseTLS=%v Insecure=%v", resolved2.UseTLS, resolved2.InsecureSkipVerify)
	}
	if resolved2.Port != 5985 {
		t.Errorf("expected saved port 5985 applied, got %d", resolved2.Port)
	}
}
