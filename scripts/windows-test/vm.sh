#!/usr/bin/env bash
# Runs one of the PowerShell scripts in this folder inside the Windows VM, as
# the logged-in user (so windows show on the VM's desktop), through the
# Parallels shared folder \\Mac\Home (= $HOME on the Mac).
#
#   scripts/windows-test/vm.sh ae screen
#   scripts/windows-test/vm.sh ae click drop
#   scripts/windows-test/vm.sh shot /tmp/x.png     # screenshot of the VM screen
#
# Env: VM (default "Windows 11").
set -euo pipefail
VM="${VM:-Windows 11}"
here="$(cd "$(dirname "$0")" && pwd)"
case "$here" in
  "$HOME"/*) ;;
  *) echo "the repo must be under \$HOME to be visible as \\\\Mac\\Home" >&2; exit 1 ;;
esac
winpath() { # /Users/me/x/y -> \\Mac\Home\x\y
  local rel="${1#"$HOME"/}"
  printf '\\\\Mac\\Home\\%s' "${rel//\//\\}"
}
cmd="$1"; shift
if [[ "$cmd" == shot ]]; then
  exec prlctl capture "$VM" --file "$1"
fi
exec prlctl exec "$VM" --current-user powershell -NoProfile -ExecutionPolicy Bypass \
  -File "$(winpath "$here/$cmd.ps1")" "$@"
