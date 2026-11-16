#!/usr/bin/env bash
# Fake `madmail` for the lab: records endpoint-cache calls per relay user.
set -euo pipefail
who="${SUDO_USER:-$(id -un)}"
dir=/var/lib/madmail-stub; mkdir -p "$dir"
db="$dir/$who.endpoints"; log="$dir/$who.calls.log"; touch "$db"
echo "$(date '+%T') madmail $*" >>"$log"
case "${1:-} ${2:-}" in
  "version "*) echo "madmail-v2 0.0.0-lab-stub ($who)" ;;
  "endpoint-cache set") grep -v -F -x -e "$3"$'\t'"$4" "$db" | awk -F'\t' -v k="$3" '$1!=k' >"$db.tmp" || true; printf '%s\t%s\n' "$3" "$4" >>"$db.tmp"; mv "$db.tmp" "$db"; echo "set $3 -> $4" ;;
  "endpoint-cache remove") awk -F'\t' -v k="$3" '$1!=k' "$db" >"$db.tmp" || true; mv "$db.tmp" "$db"; echo "removed $3" ;;
  "endpoint-cache list") if [ -s "$db" ]; then sed 's/\t/ -> /' "$db"; else echo "No endpoint override entries."; fi ;;
  "federation status") echo "lab stub: federation status ok" ;;
  *) echo "stub: unsupported: $*" >&2; exit 2 ;;
esac
