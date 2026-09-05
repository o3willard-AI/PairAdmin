#!/usr/bin/env bash
#
# verify-sbom.sh — reproduce and verify PairAdmin's release SBOM (R-12).
#
# The release pipeline generates ONE combined CycloneDX SBOM per release
# covering both the Go module deps (go.mod/go.sum) and the frontend npm deps
# (package-lock.json) — the frontend is compiled into each binary via go:embed,
# so a binary-only scan would miss it. This script lets a reviewer reproduce
# that SBOM from the source tree at the released tag and prove it matches the
# artifact attached to the GitHub release.
#
# Usage:
#   scripts/verify-sbom.sh <release-tag> [path/to/source-tree]
#
#   <release-tag>        the v* tag that was released, e.g. v1.4.2
#   [source-tree]        optional path to a checkout of that tag (defaults to
#                        the current working directory)
#
# It uses the SAME pinned syft version and SHA256 as release.yml; if syft is
# already installed at that version on PATH it is reused, otherwise it is
# downloaded, checksum-verified, and run from /tmp.
#
# IMPORTANT: pass a CLEAN checkout of the tag (e.g. `git clone --branch $TAG`
# or `git worktree add $TAG`) — the release pipeline generates the SBOM from a
# fresh checkout with no node_modules/ or frontend/dist/ build artifacts, and
# the reproduced scan must see the same tree to compare like-for-like.
#
# Exit 0 = SBOMs match (release SBOM is reproducible from source). Otherwise
# an explanatory message is printed and the script exits non-zero.
set -euo pipefail

SYFT_VERSION="1.51.1"
SYFT_SHA256="8fcb33017a0dc1058298c923c436d19dfa68ae93968e0b423248542e3afb9fc3"

TAG="${1:?usage: verify-sbom.sh <release-tag> [source-tree]}"
SRC="${2:-$(pwd)}"

SYFT=""
# Reuse an already-verified syft at the pinned version if present (on PATH or
# in /tmp from a prior run); otherwise download + checksum-verify + unpack.
for cand in syft /tmp/syft; do
  if command -v "$cand" >/dev/null 2>&1 || [[ -x "$cand" ]]; then
    # `syft version` prints "Version:  1.51.1"; grab that token.
    v="$("$cand" version 2>/dev/null | awk '/^Version:/{print $2}')"
    if [[ "$v" == "$SYFT_VERSION" ]]; then SYFT="$cand"; break; fi
  fi
done
if [[ -z "$SYFT" ]]; then
  echo "==> Downloading pinned syft v${SYFT_VERSION} and verifying checksum..."
  curl -fsSL -o /tmp/syft.tar.gz \
    "https://github.com/anchore/syft/releases/download/v${SYFT_VERSION}/syft_${SYFT_VERSION}_linux_amd64.tar.gz"
  echo "${SYFT_SHA256}  /tmp/syft.tar.gz" | sha256sum --check -
  tar -xzf /tmp/syft.tar.gz -C /tmp
  SYFT=/tmp/syft
fi
echo "using syft: $("$SYFT" version 2>/dev/null | head -1)"

if [[ ! -f "$SRC/go.mod" || ! -f "$SRC/frontend/package-lock.json" ]]; then
  echo "error: $SRC is not a PairAdmin source tree (missing go.mod / frontend/package-lock.json)" >&2
  exit 2
fi

VERSION="${TAG#v}"
echo "==> Reproducing SBOM for $TAG from $SRC ..."

# Run the exact command the release pipeline uses.
"$SYFT" scan dir:"$SRC" \
  --source-name pairadmin \
  --source-version "$VERSION" \
  -o cyclonedx-json > /tmp/pairadmin_reproduced_sbom.json

RELEASED="${RELEASE_SBOM:-/tmp/pairadmin_${VERSION}_sbom.json}"
if [[ ! -f "$RELEASED" ]]; then
  echo "error: released SBOM not found at '$RELEASED'." >&2
  echo "       Download it from the GitHub release (e.g. 'gh release download $TAG -p pairadmin_${VERSION}_sbom.json')," >&2
  echo "       or set RELEASE_SBOM=/path/to/pairadmin_${VERSION}_sbom.json ." >&2
  exit 2
fi

echo "==> Comparing reproduced SBOM against $RELEASED ..."
# Compare the dependency inventory, not volatile CycloneDX metadata or
# path-derived bom-refs. The timestamp, serialNumber (a fresh UUID per run)
# and the bom-ref/absolute file-path properties all change between identical
# scans run from different directories. The stable, meaningful content is the
# set of purls (package URL: namespace/name@version), which is what proves the
# two SBOMs describe the same bill of materials.
inventory() {
  python3 -c '
import json, sys
d = json.load(open(sys.argv[1]))
purls = []
for c in d.get("components", []):
    if c.get("purl"):
        purls.append(c["purl"])
for u in sorted(set(purls)):
    print(u)
' "$1"
}
inventory /tmp/pairadmin_reproduced_sbom.json > /tmp/repro_inventory.txt
inventory "$RELEASED" > /tmp/released_inventory.txt

if diff -q /tmp/repro_inventory.txt /tmp/released_inventory.txt >/dev/null; then
  echo "OK: reproduced SBOM and release SBOM list the same dependency inventory"
  echo "    ($(wc -l < /tmp/repro_inventory.txt) packages) — they describe the same bill of materials."
  exit 0
else
  echo "MISMATCH: reproduced SBOM dependency inventory differs from the release SBOM." >&2
  echo "  - Reproduced: /tmp/repro_inventory.txt" >&2
  echo "  - Released:   /tmp/released_inventory.txt" >&2
  echo "The released asset may have been built from a different tree/version," >&2
  echo "or the release SBOM may not correspond to tag $TAG." >&2
  exit 1
fi