#!/usr/bin/env sh
set -eu

INSTALL_HOME=""
BIN_DIR=""
YES=0
REMOVE_DATA=0
KEEP_DATA=0
RECEIPT_PATH=""

usage() {
	cat <<'EOF'
Usage: scripts/install/uninstall.sh [options]

Options:
  --yes              Use defaults for prompts.
  --home PATH        Yemaka data/runtime directory.
  --bin-dir PATH     Directory containing the yemaka command shim.
  --remove-data      Remove Yemaka data after deleting the command shim.
  --keep-data        Keep Yemaka data (default).
  -h, --help         Show this help.

Set YEMAKA_INSTALL_RECEIPT to override the user-local install receipt path.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
		--yes)
			YES=1
			;;
		--home)
			shift
			INSTALL_HOME="${1:-}"
			;;
		--bin-dir)
			shift
			BIN_DIR="${1:-}"
			;;
		--remove-data)
			REMOVE_DATA=1
			KEEP_DATA=0
			;;
		--keep-data)
			KEEP_DATA=1
			REMOVE_DATA=0
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "Unknown option: $1" >&2
			usage >&2
			exit 2
			;;
	esac
	shift
done

say() {
	printf '%s\n' "$*"
}

setup_colors() {
	if [ -t 1 ] && [ "${TERM:-}" != "dumb" ] && [ -z "${NO_COLOR:-}" ]; then
		ESC="$(printf '\033')"
		RESET="${ESC}[0m"
		BOLD="${ESC}[1m"
		DIM="${ESC}[2m"
		BRAND="${ESC}[38;2;61;214;140m"
		OK="${ESC}[32m"
		WARN="${ESC}[33m"
	else
		RESET=""
		BOLD=""
		DIM=""
		BRAND=""
		OK=""
		WARN=""
	fi
}

banner() {
	title="$1"
	subtitle="$2"
	printf '%s' "$BRAND$BOLD"
	cat <<'EOF'
__   __                    _
\ \ / /__ _ __ ___   __ _ | | ____ _
 \ V / _ \ '_ ` _ \ / _` || |/ / _` |
  | |  __/ | | | | | (_| ||   < (_| |
  |_|\___|_| |_| |_|\__,_||_|\_\__,_|
EOF
	printf '%s\n' "$RESET"
	say "${BOLD}$title${RESET}"
	say "${DIM}$subtitle${RESET}"
	if [ -n "${BRAND_ICON:-}" ] && [ -f "$BRAND_ICON" ]; then
		say "${DIM}Brand icon: $BRAND_ICON${RESET}"
	fi
	say ""
}

section() {
	say ""
	say "${BRAND}${BOLD}$1${RESET}"
}

kv() {
	label="$1"
	value="$2"
	printf '  %s%-24s%s %s\n' "$DIM" "$label" "$RESET" "$value"
}

ok() {
	say "${OK}OK${RESET} $*"
}

warn() {
	printf '%sWarning:%s %s\n' "$WARN" "$RESET" "$*" >&2
}

fail() {
	printf '%sError:%s %s\n' "$WARN" "$RESET" "$*" >&2
	exit 1
}

expand_path() {
	case "$1" in
		~) printf '%s\n' "$HOME" ;;
		~/*) printf '%s/%s\n' "$HOME" "${1#~/}" ;;
		*) printf '%s\n' "$1" ;;
	esac
}

absolute_path() {
	expanded="$(expand_path "$1")"
	[ -n "$expanded" ] || return 1
	case "$expanded" in
		/*) path="$expanded" ;;
		*) path="$PWD/$expanded" ;;
	esac
	parent="$(dirname -- "$path")"
	leaf="$(basename -- "$path")"
	while [ ! -d "$parent" ] && [ "$parent" != "/" ]; do
		leaf="$(basename -- "$parent")/$leaf"
		parent="$(dirname -- "$parent")"
	done
	resolved_parent="$(CDPATH= cd -- "$parent" && pwd -P)" || return 1
	if [ "$leaf" = "." ]; then
		printf '%s\n' "$resolved_parent"
	else
		printf '%s/%s\n' "$resolved_parent" "$leaf"
	fi
}

path_is_equal_or_inside() {
	parent="${1%/}"
	child="${2%/}"
	[ "$child" = "$parent" ] || case "$child" in
		"$parent"/*) return 0 ;;
		*) return 1 ;;
	esac
}

repo_root() {
	(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
}

validate_install_home() {
	path="$1"
	home_abs="$(absolute_path "$HOME")"
	repo_abs="$(repo_root)"
	case "$path" in
		""|"/"|"/Users"|"/home"|"/tmp"|"/private/tmp")
			fail "refusing unsafe Yemaka home: $path"
			;;
	esac
	if [ "$path" = "$home_abs" ]; then
		fail "refusing Yemaka home equal to your home directory: $path"
	fi
	if path_is_equal_or_inside "$path" "$repo_abs"; then
		fail "refusing Yemaka home that contains the repository: $path"
	fi
	if path_is_equal_or_inside "$repo_abs" "$path"; then
		fail "refusing Yemaka home inside the source repository: $path"
	fi
	for protected in \
		"$home_abs/Desktop" \
		"$home_abs/Documents" \
		"$home_abs/Downloads" \
		"$home_abs/Library" \
		"$home_abs/.config" \
		"$home_abs/.local" \
		"$home_abs/.local/share" \
		"/Users/$(basename -- "$home_abs")"; do
		if [ "$path" = "$protected" ]; then
			fail "refusing common user directory as Yemaka home: $path"
		fi
	done
	base_lower="$(basename -- "$path" | tr '[:upper:]' '[:lower:]')"
	case "$base_lower" in
		*yemaka*) ;;
		*) fail "Yemaka home directory name must include 'yemaka': $path" ;;
	esac
}

validate_bin_dir() {
	path="$1"
	home_abs="$(absolute_path "$HOME")"
	repo_abs="$(repo_root)"
	case "$path" in
		""|"/"|"/Users"|"/home"|"/tmp"|"/private/tmp")
			fail "refusing unsafe command shim directory: $path"
			;;
	esac
	if [ "$path" = "$home_abs" ]; then
		fail "refusing command shim directory equal to your home directory: $path"
	fi
	if path_is_equal_or_inside "$path" "$repo_abs"; then
		fail "refusing command shim directory that contains the repository: $path"
	fi
	if path_is_equal_or_inside "$repo_abs" "$path"; then
		fail "refusing command shim directory inside the source repository: $path"
	fi
}

default_home() {
	case "$(uname -s 2>/dev/null || echo unknown)" in
		Darwin) printf '%s\n' "$HOME/Library/Application Support/Yemaka" ;;
		Linux)
			if [ -n "${XDG_DATA_HOME:-}" ]; then
				printf '%s\n' "$XDG_DATA_HOME/yemaka"
			else
				printf '%s\n' "$HOME/.local/share/yemaka"
			fi
			;;
		*) printf '%s\n' "$HOME/.yemaka" ;;
	esac
}

default_bin_dir() {
	case "$(uname -s 2>/dev/null || echo unknown)" in
		Darwin)
			if [ -x "$HOME/.local/bin/yemaka" ]; then
				printf '%s\n' "$HOME/.local/bin"
			elif [ -x /opt/homebrew/bin/yemaka ]; then
				printf '%s\n' "/opt/homebrew/bin"
			elif [ -x /usr/local/bin/yemaka ]; then
				printf '%s\n' "/usr/local/bin"
			else
				printf '%s\n' "$HOME/.local/bin"
			fi
			;;
		Linux) printf '%s\n' "$HOME/.local/bin" ;;
		*) printf '%s\n' "$HOME/.local/bin" ;;
	esac
}

install_receipt_path() {
	if [ -n "${YEMAKA_INSTALL_RECEIPT:-}" ]; then
		printf '%s\n' "$(absolute_path "$YEMAKA_INSTALL_RECEIPT")"
		return
	fi
	case "$(uname -s 2>/dev/null || echo unknown)" in
		Darwin)
			printf '%s\n' "$HOME/Library/Application Support/Yemaka/Installer/install-receipt.env"
			;;
		Linux)
			if [ -n "${XDG_STATE_HOME:-}" ]; then
				printf '%s\n' "$XDG_STATE_HOME/yemaka/install-receipt.env"
			else
				printf '%s\n' "$HOME/.local/state/yemaka/install-receipt.env"
			fi
			;;
		*)
			printf '%s\n' "$HOME/.yemaka/install-receipt.env"
			;;
	esac
}

receipt_get() {
	key="$1"
	file="$2"
	[ -f "$file" ] || return 0
	sed -n "s/^$key=//p" "$file" | tail -n 1
}

remove_receipt_if_current() {
	[ -n "$RECEIPT_PATH" ] || return 0
	[ -f "$RECEIPT_PATH" ] || return 0
	recorded_home="$(receipt_get install_home "$RECEIPT_PATH")"
	if [ "$recorded_home" = "$INSTALL_HOME" ]; then
		rm -f "$RECEIPT_PATH"
		rmdir "$(dirname -- "$RECEIPT_PATH")" 2>/dev/null || true
		ok "Removed installation receipt: $RECEIPT_PATH"
	fi
}

ask() {
	prompt="$1"
	default="$2"
	if [ "$YES" -eq 1 ]; then
		printf '%s\n' "$default"
		return
	fi
	printf '%s%s%s [%s]: ' "$BRAND" "$prompt" "$RESET" "$default" >&2
	read -r answer || answer=""
	if [ -z "$answer" ]; then
		answer="$default"
	fi
	printf '%s\n' "$answer"
}

confirm() {
	answer="$(ask "$1" "$2")"
	case "$(printf '%s' "$answer" | tr '[:upper:]' '[:lower:]')" in
		y|yes|true|1) return 0 ;;
		*) return 1 ;;
	esac
}

RECEIPT_PATH="$(install_receipt_path)"
RECORDED_INSTALL_HOME=""
RECORDED_BIN_DIR=""
RECORDED_INSTALL_BIN=""
RECORDED_SHIM=""
RECORDED_BRAND_ICON=""
if [ -f "$RECEIPT_PATH" ]; then
	RECORDED_INSTALL_HOME="$(receipt_get install_home "$RECEIPT_PATH")"
	RECORDED_BIN_DIR="$(receipt_get bin_dir "$RECEIPT_PATH")"
	RECORDED_INSTALL_BIN="$(receipt_get install_bin "$RECEIPT_PATH")"
	RECORDED_SHIM="$(receipt_get shim "$RECEIPT_PATH")"
	RECORDED_BRAND_ICON="$(receipt_get brand_icon "$RECEIPT_PATH")"
fi
REPO_ROOT="$(repo_root)"
BRAND_ICON="$REPO_ROOT/frontend/public/favicon.svg"
if [ -n "$RECORDED_BRAND_ICON" ] && [ -f "$RECORDED_BRAND_ICON" ]; then
	BRAND_ICON="$RECORDED_BRAND_ICON"
elif [ -n "$RECORDED_INSTALL_HOME" ] && [ -f "$RECORDED_INSTALL_HOME/brand/favicon.svg" ]; then
	BRAND_ICON="$RECORDED_INSTALL_HOME/brand/favicon.svg"
fi
setup_colors

banner "Uninstaller" "Remove the command shim and binary. Local data is kept unless you approve data removal."

if [ -z "$INSTALL_HOME" ] && [ -n "${YEMAKA_HOME:-}" ]; then
	INSTALL_HOME="$YEMAKA_HOME"
fi
if [ -z "$INSTALL_HOME" ]; then
	default_install_home="$RECORDED_INSTALL_HOME"
	if [ -z "$default_install_home" ]; then
		default_install_home="$(default_home)"
	fi
	INSTALL_HOME="$(ask "Which Yemaka home should be checked?" "$default_install_home")"
fi
INSTALL_HOME="$(absolute_path "$INSTALL_HOME")" || fail "could not resolve Yemaka home"
validate_install_home "$INSTALL_HOME"
if [ -z "$BIN_DIR" ]; then
	default_bin_dir_value="$RECORDED_BIN_DIR"
	if [ -z "$default_bin_dir_value" ]; then
		default_bin_dir_value="$(default_bin_dir)"
	fi
	BIN_DIR="$(ask "Where is the yemaka command installed?" "$default_bin_dir_value")"
fi
BIN_DIR="$(absolute_path "$BIN_DIR")" || fail "could not resolve command shim directory"
validate_bin_dir "$BIN_DIR"

section "Uninstall Plan"
if [ -f "$RECEIPT_PATH" ]; then
	kv "Install receipt" "$RECEIPT_PATH"
	kv "Recorded home" "${RECORDED_INSTALL_HOME:-unknown}"
	kv "Recorded command dir" "${RECORDED_BIN_DIR:-unknown}"
else
	warn "No installation receipt found. Using prompts and platform defaults."
fi
kv "Yemaka home" "$INSTALL_HOME"
kv "Command directory" "$BIN_DIR"

if ! confirm "Remove the Yemaka command and installed binary?" "yes"; then
	say "Cancelled."
	exit 0
fi

SHIM="$BIN_DIR/yemaka"
if [ -e "$SHIM" ]; then
	rm -f "$SHIM"
	ok "Removed command shim: $SHIM"
elif [ -n "$RECORDED_SHIM" ] && [ -e "$RECORDED_SHIM" ]; then
	rm -f "$RECORDED_SHIM"
	ok "Removed recorded command shim: $RECORDED_SHIM"
else
	say "No command shim found at: $SHIM"
fi

if [ -e "$INSTALL_HOME/libexec/yemaka" ]; then
	rm -f "$INSTALL_HOME/libexec/yemaka"
	ok "Removed installed binary: $INSTALL_HOME/libexec/yemaka"
elif [ -n "$RECORDED_INSTALL_BIN" ] && [ -e "$RECORDED_INSTALL_BIN" ]; then
	rm -f "$RECORDED_INSTALL_BIN"
	ok "Removed recorded installed binary: $RECORDED_INSTALL_BIN"
fi

if [ -e "$INSTALL_HOME/bin/yemaka" ]; then
	rm -f "$INSTALL_HOME/bin/yemaka"
	ok "Removed legacy installed binary: $INSTALL_HOME/bin/yemaka"
fi

if [ "$REMOVE_DATA" -eq 0 ] && [ "$KEEP_DATA" -eq 0 ]; then
	if confirm "Also remove local data, config, memory, profiles, logs, and assets?" "no"; then
		REMOVE_DATA=1
	fi
fi

if [ "$REMOVE_DATA" -eq 1 ]; then
	if [ ! -f "$INSTALL_HOME/.yemaka-install-root" ]; then
		fail "refusing to remove data without Yemaka install marker: $INSTALL_HOME/.yemaka-install-root"
	fi
	warn "Removing only the marked Yemaka install home: $INSTALL_HOME"
	rm -rf "$INSTALL_HOME"
	ok "Removed Yemaka data directory: $INSTALL_HOME"
	remove_receipt_if_current
else
	ok "Kept Yemaka data directory: $INSTALL_HOME"
fi
