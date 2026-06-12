#!/usr/bin/env sh
set -eu

INSTALL_HOME=""
BIN_DIR=""
YES=0
REMOVE_DATA=0
KEEP_DATA=0

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

warn() {
	printf 'Warning: %s\n' "$*" >&2
}

fail() {
	printf 'Error: %s\n' "$*" >&2
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
			if [ -x /opt/homebrew/bin/yemaka ]; then
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

ask() {
	prompt="$1"
	default="$2"
	if [ "$YES" -eq 1 ]; then
		printf '%s\n' "$default"
		return
	fi
	printf '%s [%s]: ' "$prompt" "$default" >&2
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

if [ -z "$INSTALL_HOME" ]; then
	INSTALL_HOME="$(ask "Yemaka data directory" "$(default_home)")"
fi
INSTALL_HOME="$(absolute_path "$INSTALL_HOME")" || fail "could not resolve Yemaka home"
validate_install_home "$INSTALL_HOME"
if [ -z "$BIN_DIR" ]; then
	BIN_DIR="$(ask "Command shim directory" "$(default_bin_dir)")"
fi
BIN_DIR="$(absolute_path "$BIN_DIR")" || fail "could not resolve command shim directory"
validate_bin_dir "$BIN_DIR"

say "Yemaka uninstall"
say "Yemaka home: $INSTALL_HOME"
say "Command shim directory: $BIN_DIR"

if ! confirm "Remove the Yemaka command shim/binary from this install?" "yes"; then
	say "Cancelled."
	exit 0
fi

SHIM="$BIN_DIR/yemaka"
if [ -e "$SHIM" ]; then
	rm -f "$SHIM"
	say "Removed command shim: $SHIM"
else
	say "No command shim found at: $SHIM"
fi

if [ -e "$INSTALL_HOME/libexec/yemaka" ]; then
	rm -f "$INSTALL_HOME/libexec/yemaka"
	say "Removed installed binary: $INSTALL_HOME/libexec/yemaka"
fi

if [ -e "$INSTALL_HOME/bin/yemaka" ]; then
	rm -f "$INSTALL_HOME/bin/yemaka"
	say "Removed legacy installed binary: $INSTALL_HOME/bin/yemaka"
fi

if [ "$REMOVE_DATA" -eq 0 ] && [ "$KEEP_DATA" -eq 0 ]; then
	if confirm "Remove Yemaka data, config, memory, profiles, logs, and web assets?" "no"; then
		REMOVE_DATA=1
	fi
fi

if [ "$REMOVE_DATA" -eq 1 ]; then
	if [ ! -f "$INSTALL_HOME/.yemaka-install-root" ]; then
		fail "refusing to remove data without Yemaka install marker: $INSTALL_HOME/.yemaka-install-root"
	fi
	warn "Removing only the marked Yemaka install home: $INSTALL_HOME"
	rm -rf "$INSTALL_HOME"
	say "Removed Yemaka data directory: $INSTALL_HOME"
else
	say "Kept Yemaka data directory: $INSTALL_HOME"
fi
