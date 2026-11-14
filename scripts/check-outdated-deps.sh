#!/usr/bin/env bash
# Check for outdated Go modules and npm dependencies.
#
# Usage:
#   scripts/check-outdated-deps.sh            # strict: exit 1 on outdated direct deps
#   scripts/check-outdated-deps.sh --warn     # report only, always exit 0
#   scripts/check-outdated-deps.sh --staged   # run only when dependency files are staged
#
# Set SKIP_DEPS_CHECK=1 to bypass the check entirely (for example on a plane).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MODE="strict"
for arg in "$@"; do
  case "$arg" in
    --strict) MODE="strict" ;;
    --warn) MODE="warn" ;;
    --staged) MODE="staged" ;;
    -h | --help)
      sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "Unknown option: $arg" >&2
      exit 2
      ;;
  esac
done

if [[ "${SKIP_DEPS_CHECK:-0}" != "0" ]]; then
  echo "Dependency freshness check skipped (SKIP_DEPS_CHECK is set)."
  exit 0
fi

if [[ "$MODE" == "staged" ]]; then
  if ! git diff --cached --name-only --diff-filter=ACMR | grep -Eq '^(go\.(mod|sum)|frontend/package(-lock)?\.json)$'; then
    exit 0
  fi
  # Never block a commit on network results; report and let the push gate decide.
  MODE="warn"
fi

if [[ -t 1 ]]; then
  BOLD="$(printf '\033[1m')"
  DIM="$(printf '\033[2m')"
  YELLOW="$(printf '\033[33m')"
  GREEN="$(printf '\033[32m')"
  RESET="$(printf '\033[0m')"
else
  BOLD="" DIM="" YELLOW="" GREEN="" RESET=""
fi

echo "${BOLD}Dependency freshness${RESET} ${DIM}($ROOT)${RESET}"

go_direct=0
go_indirect=0
go_total=0
npm_direct=0
npm_total=0

if command -v go >/dev/null 2>&1; then
  echo
  echo "${BOLD}Go modules${RESET}"
  # -u asks the proxy for the latest version of each module. Template output is
  # tab-separated so module paths with unusual characters stay intact.
  go_updates="$(
    go list -u -m -f '{{if and (not .Main) .Update}}{{.Path}}|{{.Version}}|{{.Update.Version}}|{{.Indirect}}{{end}}' all 2>/dev/null || true
  )"
  if [[ -n "$go_updates" ]]; then
    while IFS='|' read -r module current latest indirect; do
      [[ -z "${module:-}" ]] && continue
      go_total=$((go_total + 1))
      if [[ "$indirect" == "true" ]]; then
        go_indirect=$((go_indirect + 1))
        printf '  %s~ %s %s -> %s%s %s(indirect)%s\n' "$DIM" "$module" "$current" "$latest" "$RESET" "$DIM" "$RESET"
      else
        go_direct=$((go_direct + 1))
        printf '  %s! %s %s -> %s%s\n' "$YELLOW" "$module" "$current" "$latest" "$RESET"
      fi
    done <<<"$go_updates"
  else
    echo "  ${GREEN}all modules are current${RESET}"
  fi
  if [[ "$go_total" -gt 0 ]]; then
    echo "  ${DIM}$go_direct direct, $go_indirect indirect update(s) available${RESET}"
  fi
else
  echo
  echo "${YELLOW}Go is not installed; skipping Go module check.${RESET}"
fi

if [[ -d frontend/node_modules ]] && command -v npm >/dev/null 2>&1 && command -v node >/dev/null 2>&1; then
  echo
  echo "${BOLD}npm packages${RESET}"
  npm_json="$(mktemp)"
  trap 'rm -f "$npm_json"' EXIT
  npm --prefix frontend outdated --json >"$npm_json" 2>/dev/null || true
  if node - "$npm_json" <<'NODE'; then
const fs = require("node:fs");
const path = process.argv[2];
let data = {};
try {
  data = JSON.parse(fs.readFileSync(path, "utf8") || "{}");
} catch {
  data = {};
}
const pkg = JSON.parse(fs.readFileSync("frontend/package.json", "utf8"));
const direct = new Set([
  ...Object.keys(pkg.dependencies || {}),
  ...Object.keys(pkg.devDependencies || {}),
]);
const entries = Object.entries(data);
const dim = process.stdout.isTTY ? "\u001b[2m" : "";
const yellow = process.stdout.isTTY ? "\u001b[33m" : "";
const green = process.stdout.isTTY ? "\u001b[32m" : "";
const reset = process.stdout.isTTY ? "\u001b[0m" : "";
let directOutdated = 0;
let indirectOutdated = 0;
if (entries.length === 0) {
  console.log(`  ${green}all packages are current${reset}`);
  process.exit(0);
}
for (const [name, info] of entries) {
  const isDirect = direct.has(name);
  const current = info.current || "?";
  const wanted = info.wanted || current;
  const latest = info.latest || wanted;
  const latestNote = latest !== wanted ? ` (latest ${latest})` : "";
  if (isDirect) {
    directOutdated += 1;
    console.log(`  ${yellow}! ${name} ${current} -> ${wanted}${latestNote}${reset}`);
  } else {
    indirectOutdated += 1;
    console.log(`  ${dim}~ ${name} ${current} -> ${wanted}${latestNote}${reset}`);
  }
}
console.log(
  `  ${dim}${directOutdated} direct, ${indirectOutdated} transitive update(s) available${reset}`,
);
process.exit(directOutdated > 0 ? 10 : 0);
NODE
    npm_direct=0
  else
    status=$?
    if [[ "$status" == "10" ]]; then
      npm_direct=1
    else
      echo "  ${YELLOW}npm outdated failed with exit code $status${RESET}"
    fi
  fi
else
  echo
  echo "${DIM}npm check skipped (install frontend dependencies first).${RESET}"
fi

echo
if [[ "$go_direct" -gt 0 || "$npm_direct" -gt 0 ]]; then
  if [[ "$MODE" == "strict" ]]; then
    echo "${YELLOW}Outdated direct dependencies found.${RESET}"
    echo "Update them before pushing, or bypass this gate with:"
    echo "  SKIP_DEPS_CHECK=1 git push"
    exit 1
  fi
  echo "${YELLOW}Outdated direct dependencies found (not blocking in warn mode).${RESET}"
else
  echo "${GREEN}Direct dependencies are current.${RESET}"
fi
