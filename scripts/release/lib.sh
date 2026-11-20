# Shared by the release scripts. Sourced, not run.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
APP_NAME="AudioExtractor"
DIST="${REPO_ROOT}/dist"
BIN="${REPO_ROOT}/bin"
NOTICE="${REPO_ROOT}/THIRD_PARTY_NOTICES.txt"
NOTES="${REPO_ROOT}/docs/first-launch"
# The Taskfile passes the version it stamped into the binaries.
VERSION="${VERSION:-$(cd "$REPO_ROOT" && git describe --tags --always --dirty 2>/dev/null || echo dev)}"

log() { printf '\n==> %s\n' "$*" >&2; }
die() { printf 'release: %s\n' "$*" >&2; exit 1; }

# Fresh staging folder (the caller removes it when done).
stage() { rm -rf "$1"; mkdir -p "$1"; }
