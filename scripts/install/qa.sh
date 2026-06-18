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

install_home="$tmp_root/yemaka-installed-home"
install_bin="$tmp_root/yemaka-installed-bin"
outside_repo="$tmp_root/outside-repo"
install_receipt="$tmp_root/install-receipt.env"
install_home_abs="$(CDPATH= cd -- "$(dirname -- "$install_home")" && pwd -P)/$(basename -- "$install_home")"
mkdir -p "$outside_repo"
expect_success install_cli env YEMAKA_INSTALL_RECEIPT="$install_receipt" scripts/install/install.sh --yes --type cli --home "$install_home" --bin-dir "$install_bin" --no-symlink --skip-frontend-build
[ -f "$install_home/packs/templates/personal_productivity/pack.yaml" ] || fail "installed domain pack templates missing"
[ -f "$install_home/skills/default/project_explainer/skill.yaml" ] || fail "installed default skills missing"
[ -f "$install_home/brand/favicon.svg" ] || fail "installed brand favicon missing"
[ -f "$install_receipt" ] || fail "install receipt missing"
grep -q "install_home=$install_home_abs" "$install_receipt" || fail "install receipt missing install home"
grep -q "brand_icon=$install_home_abs/brand/favicon.svg" "$install_receipt" || fail "install receipt missing brand icon"
expect_success installed_templates_outside_repo sh -c 'cd "$1" && YEMAKA_HOME="$2" "$3" domain-pack templates' sh "$outside_repo" "$install_home" "$install_home/libexec/yemaka"
grep -q "personal_productivity" "$log_dir/installed_templates_outside_repo.out" || fail "installed binary did not list built-in templates outside repo"
expect_success installed_default_skills_outside_repo sh -c 'cd "$1" && YEMAKA_HOME="$2" "$3" skill list' sh "$outside_repo" "$install_home" "$install_home/libexec/yemaka"
grep -q "project_explainer" "$log_dir/installed_default_skills_outside_repo.out" || fail "installed binary did not list default skills outside repo"
expect_success uninstall_uses_receipt env YEMAKA_INSTALL_RECEIPT="$install_receipt" scripts/install/uninstall.sh --yes --keep-data

if grep "ollama pull" scripts/install/install.sh scripts/install/install.ps1 scripts/install/doctor.sh scripts/install/uninstall.sh >/dev/null 2>&1; then
	fail "installer must not auto-pull Ollama models"
fi
pass "no model auto-pull in install scripts"

for script in scripts/install/install.sh scripts/install/install.ps1; do
	grep -q "No cloud, internet/search, connectors, embeddings, background jobs, or model downloads were enabled" "$script" ||
		fail "$script does not state disabled safe defaults"
	if ! grep -q "packs/templates" "$script" && ! grep -q 'packs\\templates' "$script"; then
		fail "$script does not install built-in domain pack templates"
	fi
	if ! grep -q "skills/default" "$script" && ! grep -q 'skills\\default' "$script"; then
		fail "$script does not install built-in default skills"
	fi
	grep -q "favicon.svg" "$script" ||
		fail "$script does not reference the Yemaka favicon"
	grep -q "install-receipt.env" "$script" ||
		fail "$script does not write an install receipt"
done
for script in scripts/install/install.sh scripts/install/install.ps1; do
	grep -q "Protected Defaults" "$script" ||
		fail "$script does not show protected defaults as a setup section"
	if grep -q "Keep internet/search/cloud/connectors/embeddings disabled by default" "$script"; then
		fail "$script should not ask users to override protected disabled defaults"
	fi
done
pass "install scripts state disabled safe defaults, bundled skills/templates, and receipts"

grep -q "Assert-SafeInstallHome" scripts/install/install.ps1 || fail "Windows installer lacks safe install-home validation"
grep -q ".yemaka-install-root" scripts/install/install.ps1 || fail "Windows installer lacks install marker"
grep -q "NoShim" scripts/install/install.ps1 || fail "Windows installer lacks no-shim option"
pass "Windows installer static safety checks"

say "Installer QA completed."
