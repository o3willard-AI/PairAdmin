package capture

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"
)

// RoleTerminal is the AT-SPI2 role value for terminal emulators (ATSPI_ROLE_TERMINAL = 59).
const RoleTerminal = uint32(59)

// ObjectRef is an AT-SPI2 accessible object reference (bus name + object path).
type ObjectRef struct {
	Name string
	Path dbus.ObjectPath
}

// CacheItem represents one item from Cache.GetItems (GTK4/new signature).
// Signature: a((so)(so)(so)iiassusau)
type CacheItem struct {
	Ref           ObjectRef
	AppRef        ObjectRef
	ParentRef     ObjectRef
	IndexInParent int32
	ChildCount    int32
	Interfaces    []string
	Name          string
	Role          uint32
	Description   string
	StateSet      []uint32
}

// ATSPIAdapter implements TerminalAdapter using the AT-SPI2 accessibility bus.
// It discovers GNOME Terminal windows via ATSPI_ROLE_TERMINAL (role 59) and
// captures text via org.a11y.atspi.Text.GetText(0, -1).
type ATSPIAdapter struct {
	// Injectable functions for testing — all default to real D-Bus implementations.

	// getA11yAddress returns the accessibility bus address via org.a11y.Bus.GetAddress.
	getA11yAddress func() (string, error)

	// listBusNames returns unique names on the accessibility bus.
	listBusNames func() ([]string, error)

	// getCacheItems returns cached accessible objects for a given bus name.
	getCacheItems func(busName string) ([]CacheItem, error)

	// getText calls org.a11y.atspi.Text.GetText(0, -1) on the given object.
	getText func(busName string, path dbus.ObjectPath) (string, error)

	// gsettingsOutput returns the gsettings value for toolkit-accessibility.
	gsettingsOutput func(ctx context.Context, key string) (string, error)

	// closeConn is called by Close() when a connection needs to be released.
	closeConn func() error

	// a11yAddr caches the accessibility bus address from IsAvailable.
	a11yAddr string
}

// NewATSPIAdapter creates an ATSPIAdapter with default (real D-Bus) implementations.
func NewATSPIAdapter() *ATSPIAdapter {
	a := &ATSPIAdapter{}
	// Wire up real implementations that reference 'a' after construction.
	a.getA11yAddress = a.defaultGetA11yAddress
	a.listBusNames = a.defaultListBusNames
	a.getCacheItems = a.defaultGetCacheItems
	a.getText = a.defaultGetText
	a.gsettingsOutput = defaultGsettingsOutput
	return a
}

// defaultGetA11yAddress is the real implementation: connects to session bus,
// calls org.a11y.Bus.GetAddress, and returns the accessibility bus socket address.
func (a *ATSPIAdapter) defaultGetA11yAddress() (string, error) {
	sessionConn, err := dbus.SessionBus()
	if err != nil {
		return "", fmt.Errorf("session bus unavailable: %w", err)
	}
	var addr string
	obj := sessionConn.Object("org.a11y.Bus", "/org/a11y/bus")
	if err := obj.Call("org.a11y.Bus.GetAddress", 0).Store(&addr); err != nil {
		return "", fmt.Errorf("a11y bus GetAddress failed: %w", err)
	}
	return addr, nil
}

// connectToA11yBus returns a connected accessibility bus connection using the stored a11yAddr.
func (a *ATSPIAdapter) connectToA11yBus() (*dbus.Conn, error) {
	if a.a11yAddr == "" {
		return nil, fmt.Errorf("accessibility bus address not known; call IsAvailable first")
	}
	conn, err := dbus.Dial(a.a11yAddr)
	if err != nil {
		return nil, fmt.Errorf("dial a11y bus: %w", err)
	}
	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("a11y bus auth: %w", err)
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("a11y bus hello: %w", err)
	}
	return conn, nil
}

// defaultListBusNames connects to the accessibility bus and enumerates unique names.
func (a *ATSPIAdapter) defaultListBusNames() ([]string, error) {
	conn, err := a.connectToA11yBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var names []string
	obj := conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	if err := obj.Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return nil, fmt.Errorf("ListNames failed: %w", err)
	}
	// Filter to unique names only (starting with ':') — skip well-known names.
	var unique []string
	for _, n := range names {
		if strings.HasPrefix(n, ":") {
			unique = append(unique, n)
		}
	}
	return unique, nil
}

// defaultGetCacheItems tries GTK4 Cache.GetItems signature for the given bus name.
func (a *ATSPIAdapter) defaultGetCacheItems(busName string) ([]CacheItem, error) {
	conn, err := a.connectToA11yBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	obj := conn.Object(busName, "/org/a11y/atspi/cache")
	var items []CacheItem
	call := obj.Call("org.a11y.atspi.Cache.GetItems", 0)
	if call.Err != nil {
		return nil, call.Err
	}
	if err := call.Store(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// defaultGetText calls org.a11y.atspi.Text.GetText(0, -1) on the given object.
func (a *ATSPIAdapter) defaultGetText(busName string, path dbus.ObjectPath) (string, error) {
	conn, err := a.connectToA11yBus()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	obj := conn.Object(busName, path)
	var text string
	if err := obj.Call("org.a11y.atspi.Text.GetText", 0, int32(0), int32(-1)).Store(&text); err != nil {
		return "", fmt.Errorf("GetText failed: %w", err)
	}
	return text, nil
}

// defaultGsettingsOutput runs gsettings to get the toolkit-accessibility value.
func defaultGsettingsOutput(ctx context.Context, key string) (string, error) {
	cmd := exec.CommandContext(ctx, "gsettings", "get", "org.gnome.desktop.interface", key)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Name returns the adapter type name.
func (a *ATSPIAdapter) Name() string { return "atspi" }

// IsAvailable returns true if the AT-SPI2 accessibility bus is reachable.
// It calls org.a11y.Bus.GetAddress; if a non-empty address is returned, the bus
// is operational. The address is cached for subsequent connectToA11yBus calls.
// Note: We do NOT check IsEnabled — the bus works even when GSettings is false.
func (a *ATSPIAdapter) IsAvailable(ctx context.Context) bool {
	addr, err := a.getA11yAddress()
	if err != nil || addr == "" {
		return false
	}
	a.a11yAddr = addr
	return true
}

// OnboardingRequired returns true when gsettings toolkit-accessibility is not "true".
// This means GNOME Terminal will NOT attach to the accessibility bus on startup,
// so the user needs to enable accessibility and restart their terminal.
func (a *ATSPIAdapter) OnboardingRequired(ctx context.Context) bool {
	out, err := a.gsettingsOutput(ctx, "toolkit-accessibility")
	if err != nil {
		return true // assume onboarding needed if we can't check
	}
	return strings.TrimSpace(out) != "true"
}

// Discover enumerates all unique names on the accessibility bus, calls
// Cache.GetItems for each, and returns PaneInfo for items with Role == RoleTerminal.
func (a *ATSPIAdapter) Discover(ctx context.Context) ([]PaneInfo, error) {
	names, err := a.listBusNames()
	if err != nil {
		return nil, nil // no accessible bus names — not an error, just empty
	}

	var panes []PaneInfo
	for _, busName := range names {
		items, err := a.getCacheItems(busName)
		if err != nil {
			continue // skip names that fail — not an error
		}
		for _, item := range items {
			if item.Role != RoleTerminal {
				continue
			}
			id := fmt.Sprintf("atspi:%s%s", item.Ref.Name, string(item.Ref.Path))
			name := item.Name
			if name == "" {
				name = busName
			}
			panes = append(panes, PaneInfo{
				ID:          id,
				AdapterType: "atspi",
				DisplayName: name,
			})
		}
	}
	return panes, nil
}

// Capture retrieves visible content from the given AT-SPI2 terminal pane.
// It parses the pane ID to extract bus name and object path, calls GetText(0, -1),
// then applies the ANSI + credential filter pipeline.
func (a *ATSPIAdapter) Capture(ctx context.Context, pane PaneInfo) (string, error) {
	// Parse "atspi:<busName><path>" — busName starts with ':' (unique name)
	raw := strings.TrimPrefix(pane.ID, "atspi:")
	// Bus name is the unique name starting with ':'
	// Path is everything from the first '/' character
	slashIdx := strings.Index(raw, "/")
	var busName string
	var objPath dbus.ObjectPath
	if slashIdx < 0 {
		busName = raw
		objPath = "/"
	} else {
		busName = raw[:slashIdx]
		objPath = dbus.ObjectPath(raw[slashIdx:])
	}

	text, err := a.getText(busName, objPath)
	if err != nil {
		return "", fmt.Errorf("GetText failed for pane %s: %w", pane.ID, err)
	}

	return applyFilterPipeline(text), nil
}

// Close releases the accessibility bus connection if held.
func (a *ATSPIAdapter) Close() error {
	if a.closeConn != nil {
		return a.closeConn()
	}
	return nil
}
