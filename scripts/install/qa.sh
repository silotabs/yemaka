#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$repo_root"

tmp_root="${TMPDIR:-/tmp}/yemaka-install-qa.$$"
safe_home="$tmp_root/yemaka-home"
safe_bin="$tmp_root/yemaka-bin"
log_dir="$tmp_root/logs"
mkdir -p "$log_dir"

cleanup() {
	rm -rf "$tmp_root"
}
trap cleanup EXIT INT TERM

say() {
	printf '%s\n' "$*"
}

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	exit 1
}

pass() {
	say "PASS: $*"
}

expect_success() {
	name="$1"
	shift
	if "$@" >"$log_dir/$name.out" 2>"$log_dir/$name.err"; then
		pass "$name"
	else
		cat "$log_dir/$name.err" >&2 || true
		fail "$name"
	fi
}

expect_failure() {
	name="$1"
	shift
	if "$@" >"$log_dir/$name.out" 2>"$log_dir/$name.err"; then
		cat "$log_dir/$name.out" >&2 || true
		fail "$name unexpectedly succeeded"
	else
		pass "$name"
	fi
}

say "Yemaka installer QA"
say "Repository: $repo_root"
say "Temp root: $tmp_root"

expect_success install_help scripts/install/install.sh --help
expect_success uninstall_help scripts/install/uninstall.sh --help
expect_success doctor_help scripts/install/doctor.sh --help

expect_failure reject_home_dir scripts/install/install.sh --yes --type cli --home "$HOME" --bin-dir "$safe_bin" --no-symlink --skip-frontend-build
expect_failure reject_repo_home scripts/install/install.sh --yes --type cli --home "$repo_root/.install-yemaka" --bin-dir "$safe_bin" --no-symlink --skip-frontend-build
expect_failure reject_unsafe_bin scripts/install/install.sh --yes --type cli --home "$safe_home" --bin-dir "$HOME" --no-symlink --skip-frontend-build

mkdir -p "$safe_home" "$safe_bin"
expect_failure uninstall_refuses_unmarked_data scripts/install/uninstall.sh --yes --home "$safe_home" --bin-dir "$safe_bin" --remove-data

if grep "ollama pull" scripts/install/install.sh scripts/install/install.ps1 scripts/install/doctor.sh scripts/install/uninstall.sh >/dev/null 2>&1; then
	fail "installer must not auto-pull Ollama models"
fi
pass "no model auto-pull in install scripts"

for script in scripts/install/install.sh scripts/install/install.ps1; do
	grep -q "No cloud, internet/search, connectors, embeddings, background jobs, or model downloads were enabled" "$script" ||
		fail "$script does not state disabled safe defaults"
done
pass "install scripts state disabled safe defaults"

grep -q "Assert-SafeInstallHome" scripts/install/install.ps1 || fail "Windows installer lacks safe install-home validation"
grep -q ".yemaka-install-root" scripts/install/install.ps1 || fail "Windows installer lacks install marker"
grep -q "NoShim" scripts/install/install.ps1 || fail "Windows installer lacks no-shim option"
pass "Windows installer static safety checks"

say "Installer QA completed."
