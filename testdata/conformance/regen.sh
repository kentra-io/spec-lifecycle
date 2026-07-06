#!/usr/bin/env bash
#
# Regenerate testdata/conformance/ from the real reference oracle
# (@fission-ai/openspec@1.5.0). This is a DOCUMENTATION / reproducibility
# script, not something CI runs — it needs network access (npm install +
# git clone) and its output has two categories of non-determinism you must
# expect and discard:
#
#   1. `openspec archive` timestamps the archived folder name with TODAY's
#      date (`YYYY-MM-DD-<change-name>`) — cosmetic, per spec-lifecycle.md
#      §4 errata. We never capture the archive/ folder itself, only the
#      pre-archive `before/`+`change/` and post-archive `expected/specs/`.
#   2. Absolute tmp paths differ per run. Nothing we capture into the repo
#      contains an absolute path (we deliberately do NOT keep the oracle's
#      raw JSON stdout, which embeds one — see README.md "What we did not
#      keep").
#
# Regenerate ONLY as a deliberate, explicit decision to track a newer
# OpenSpec grammar version (spec-lifecycle.md §12.1/§13). Bumping the
# pinned version below is that decision.
#
# Usage:
#   ./regen.sh                  # regenerate all cases into ./cases/
#   ./regen.sh 05               # regenerate only case matching "05-*"
#
# Requires: node >=20, npm, git, python3 (for manifest.json), network access.

set -euo pipefail

ORACLE_VERSION="1.5.0"
ORACLE_TAG="v1.5.0"
ORACLE_COMMIT="546224e00db26bd1be69874be465d5d6f5e4a851" # expected HEAD of the tag, verified at capture time

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CASES_DIR="$SCRIPT_DIR/cases"
WORK=$(mktemp -d /tmp/openspec-conformance-regen.XXXXXX)
trap 'rm -rf "$WORK"' EXIT

echo "== Installing oracle @fission-ai/openspec@${ORACLE_VERSION} =="
mkdir -p "$WORK/oracle"
(cd "$WORK/oracle" && npm install "@fission-ai/openspec@${ORACLE_VERSION}" >/dev/null)
OPENSPEC_BIN="$WORK/oracle/node_modules/.bin/openspec"
ACTUAL_VERSION="$("$OPENSPEC_BIN" --version)"
if [ "$ACTUAL_VERSION" != "$ORACLE_VERSION" ]; then
  echo "FATAL: installed oracle version $ACTUAL_VERSION != pinned $ORACLE_VERSION" >&2
  exit 1
fi

echo "== Cloning OpenSpec source @ ${ORACLE_TAG} (for grammar/behavior cross-reference only) =="
git clone --quiet --depth 1 --branch "$ORACLE_TAG" https://github.com/Fission-AI/OpenSpec "$WORK/src"
ACTUAL_COMMIT="$(git -C "$WORK/src" rev-parse HEAD)"
if [ "$ACTUAL_COMMIT" != "$ORACLE_COMMIT" ]; then
  echo "WARNING: source commit $ACTUAL_COMMIT != expected $ORACLE_COMMIT (tag may have moved)" >&2
fi

# --- helpers (mirror of the ones used to capture this corpus originally) ---
snapshot_before() {
  local root="$1"
  mkdir -p "$root/before/specs"
  if [ -d "$root/openspec/specs" ]; then
    cp -r "$root/openspec/specs/." "$root/before/specs/" 2>/dev/null || true
  fi
  # git can't track empty dirs: fold-from-empty cases need a placeholder
  find "$root/before" -type d -empty -exec touch {}/.gitkeep \;
}
snapshot_change() {
  local root="$1" change="$2"
  mkdir -p "$root/change"
  cp -r "$root/openspec/changes/$change" "$root/change/"
}
run_archive() {
  local root="$1" change="$2"
  (
    cd "$root"
    set +e
    "$OPENSPEC_BIN" archive "$change" --json --yes > "$root/.oracle-stdout.json" 2>"$root/.oracle-stderr.txt"
    echo $? > "$root/.oracle-exit-code.txt"
    set -e
  )
}
snapshot_expected() {
  local root="$1"
  mkdir -p "$root/expected/specs"
  if [ -d "$root/openspec/specs" ]; then
    cp -r "$root/openspec/specs/." "$root/expected/specs/" 2>/dev/null || true
  fi
}

echo
echo "MANUAL STEP (not automated by this script):"
echo "  Each case's before/ and change/ fixtures under $CASES_DIR were hand-authored"
echo "  once (see README.md 'How each case was built') to exercise a specific fold"
echo "  edge case. This script does NOT regenerate the fixture content itself — it"
echo "  documents how to re-run the oracle against EXISTING before/+change/ fixtures"
echo "  to refresh expected/ if you deliberately choose to re-pin a newer grammar."
echo
echo "  For each case directory under cases/<NN-slug>/, this script:"
echo "    1. builds a scratch openspec/ tree from before/ + change/"
echo "    2. runs '$OPENSPEC_BIN archive <change> --json --yes'"
echo "    3. overwrites expected/specs/ with the oracle's post-archive output"
echo "    4. re-hashes everything into manifest.json"
echo

FILTER="${1:-}"
for case_dir in "$CASES_DIR"/*/; do
  case_name="$(basename "$case_dir")"
  if [ -n "$FILTER" ] && [[ "$case_name" != "$FILTER"* ]]; then
    continue
  fi
  echo "-- regenerating $case_name --"

  scratch="$WORK/scratch/$case_name"
  mkdir -p "$scratch/openspec/specs" "$scratch/openspec/changes"

  # Seed pre-archive state from the checked-in fixtures.
  if [ -d "$case_dir/before/specs" ]; then
    cp -r "$case_dir/before/specs/." "$scratch/openspec/specs/"
    find "$scratch/openspec/specs" -name .gitkeep -delete
  fi
  change_name="$(ls "$case_dir/change")"
  cp -r "$case_dir/change/$change_name" "$scratch/openspec/changes/$change_name"

  run_archive "$scratch" "$change_name"
  exit_code="$(cat "$scratch/.oracle-exit-code.txt")"
  if [ "$exit_code" != "0" ]; then
    echo "FATAL: oracle archive failed for $case_name (exit $exit_code):" >&2
    cat "$scratch/.oracle-stdout.json" >&2
    cat "$scratch/.oracle-stderr.txt" >&2
    exit 1
  fi

  rm -rf "$case_dir/expected"
  snapshot_expected "$scratch"
  cp -r "$scratch/expected" "$case_dir/expected"
done

echo "== Re-hashing manifest.json =="
python3 - "$SCRIPT_DIR" "$ORACLE_VERSION" "$ORACLE_TAG" "$ORACLE_COMMIT" <<'PYEOF'
import hashlib, json, os, sys

script_dir, version, tag, commit = sys.argv[1:5]
os.chdir(script_dir)

entries = []
for dirpath, dirnames, filenames in os.walk("cases"):
    dirnames.sort()
    for fn in sorted(filenames):
        p = os.path.join(dirpath, fn)
        rel = os.path.relpath(p, ".").replace(os.sep, "/")
        with open(p, "rb") as f:
            data = f.read()
        entries.append((rel, hashlib.sha256(data).hexdigest(), len(data)))

entries.sort(key=lambda e: e[0])
manifest = {
    "oracle": {
        "package": "@fission-ai/openspec",
        "version": version,
        "sourceRepo": "https://github.com/Fission-AI/OpenSpec",
        "sourceCommit": commit,
        "sourceTag": tag,
    },
    "files": [{"path": p, "sha256": h, "bytes": n} for p, h, n in entries],
}
with open("manifest.json", "w") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
print(f"{len(entries)} files hashed")
PYEOF

echo "Done. Review 'git diff testdata/conformance/' before committing."
