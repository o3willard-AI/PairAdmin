// Package version carries the application version shown in the About tab.
//
// The version is NEVER a hand-edited constant: a local/dev build reports
// "dev", and the release workflow injects the real value at build time via
//
//	-X pairadmin/services/version.appVersion=${GITHUB_REF_NAME}
//
// (see .github/workflows/release.yml, the same -ldflags pattern as the
// config.releaseBuild flag). wails.json's info.productVersion remains the
// declared source of truth the verify-version gate checks the tag against;
// this string equals the tag on every release binary with zero manual edits.
package version

// appVersion is overridden at link time by release builds. It is a var (not
// a const) precisely so -X ldflags can replace it.
var appVersion = "dev"

// GetVersion returns the application version ("dev" on local builds, the
// release tag on release binaries). Never empty.
func GetVersion() string {
	if appVersion == "" {
		return "dev" // belt and suspenders: a bad -X override must not blank the About tab
	}
	return appVersion
}
