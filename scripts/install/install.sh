#!/usr/bin/env sh
set -eu

APP_NAME="Yemaka"
PORT="7727"
INSTALL_TYPE=""
INSTALL_HOME=""
BIN_DIR=""
YES=0
CREATE_SHIM=1
SKIP_FRONTEND_BUILD=0

usage() {
	cat <<'EOF'
Usage: scripts/install/install.sh [options]

Options:
  --yes                         Use safe defaults for prompts.
  --type web|cli                Install Web + CLI/TUI or CLI/TUI only.
  --home PATH                   Yemaka data/runtime directory.
  --bin-dir PATH                Directory for the yemaka command shim.
  --no-symlink                  Do not create a command shim.
  --skip-frontend-build         Do not run npm build if frontend/dist is missing.
  --port PORT                   Port to check for yemaka serve (default 7727).
  -h, --help                    Show this help.

The installer never enables cloud, internet, connectors, embeddings, background
jobs, or model downloads by default.

Set YEMAKA_INSTALL_RECEIPT to override the user-local install receipt path.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
		--yes)
			YES=1
			;;
		--type)
			shift
			INSTALL_TYPE="${1:-}"
			;;
		--home)
			shift
			INSTALL_HOME="${1:-}"
			;;
		--bin-dir)
			shift
			BIN_DIR="${1:-}"
			;;
		--no-symlink|--no-shim)
			CREATE_SHIM=0
			;;
		--skip-frontend-build|--no-frontend-build)
			SKIP_FRONTEND_BUILD=1
			;;
		--port)
			shift
			PORT="${1:-}"
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
	if [ -n "${BRAND_ICON_SRC:-}" ] && [ -f "$BRAND_ICON_SRC" ]; then
		say "${DIM}Brand icon: $BRAND_ICON_SRC${RESET}"
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

note() {
	say "${DIM}$*${RESET}"
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

validate_install_home() {
	path="$1"
	home_abs="$(absolute_path "$HOME")"
	case "$path" in
		""|"/"|"/Users"|"/home"|"/tmp"|"/private/tmp")
			fail "refusing unsafe Yemaka home: $path"
			;;
	esac
	if [ "$path" = "$home_abs" ]; then
		fail "refusing to install into your home directory: $path"
	fi
	if path_is_equal_or_inside "$path" "$repo_root"; then
		fail "refusing install home that contains the repository: $path"
	fi
	if path_is_equal_or_inside "$repo_root" "$path"; then
		fail "refusing install home inside the source repository: $path"
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
	case "$path" in
		""|"/"|"/Users"|"/home"|"/tmp"|"/private/tmp")
			fail "refusing unsafe command shim directory: $path"
			;;
	esac
	if [ "$path" = "$home_abs" ]; then
		fail "refusing to put command shim directly in your home directory: $path"
	fi
	if path_is_equal_or_inside "$path" "$repo_root"; then
		fail "refusing command shim directory that contains the repository: $path"
	fi
	if path_is_equal_or_inside "$repo_root" "$path"; then
		fail "refusing command shim directory inside the source repository: $path"
	fi
}

default_home() {
	os_name="$(uname -s 2>/dev/null || echo unknown)"
	case "$os_name" in
		Darwin)
			printf '%s\n' "$HOME/Library/Application Support/Yemaka"
			;;
		Linux)
			if [ -n "${XDG_DATA_HOME:-}" ]; then
				printf '%s\n' "$XDG_DATA_HOME/yemaka"
			else
				printf '%s\n' "$HOME/.local/share/yemaka"
			fi
			;;
		*)
			printf '%s\n' "$HOME/.yemaka"
			;;
	esac
}

default_bin_dir() {
	os_name="$(uname -s 2>/dev/null || echo unknown)"
	case "$os_name" in
		Darwin)
			printf '%s\n' "$HOME/.local/bin"
			;;
		Linux)
			printf '%s\n' "$HOME/.local/bin"
			;;
		*)
			printf '%s\n' "$HOME/.local/bin"
			;;
	esac
}

install_receipt_path() {
	if [ -n "${YEMAKA_INSTALL_RECEIPT:-}" ]; then
		printf '%s\n' "$(absolute_path "$YEMAKA_INSTALL_RECEIPT")"
		return
	fi
	os_name="$(uname -s 2>/dev/null || echo unknown)"
	case "$os_name" in
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

write_install_receipt() {
	receipt_path="$(install_receipt_path)"
	receipt_dir="$(dirname -- "$receipt_path")"
	mkdir -p "$receipt_dir"
	{
		printf 'created_at=%s\n' "$INSTALL_CREATED_AT"
		printf 'app=%s\n' "$APP_NAME"
		printf 'install_type=%s\n' "$INSTALL_TYPE"
		printf 'install_home=%s\n' "$INSTALL_HOME"
		printf 'bin_dir=%s\n' "$BIN_DIR"
		printf 'install_bin=%s\n' "$INSTALL_BIN"
		printf 'shim=%s\n' "${SHIM:-}"
		printf 'log_file=%s\n' "$LOG_FILE"
		printf 'repo=%s\n' "$repo_root"
		printf 'brand_icon=%s\n' "${BRAND_ICON_DST:-}"
		printf 'port=%s\n' "$PORT"
	} >"$receipt_path"
	printf '%s\n' "$receipt_path"
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
	prompt="$1"
	default="$2"
	answer="$(ask "$prompt" "$default")"
	case "$(printf '%s' "$answer" | tr '[:upper:]' '[:lower:]')" in
		y|yes|true|1) return 0 ;;
		*) return 1 ;;
	esac
}

command_exists() {
	command -v "$1" >/dev/null 2>&1
}

ram_report() {
	os_name="$(uname -s 2>/dev/null || echo unknown)"
	case "$os_name" in
		Darwin)
			if command_exists sysctl; then
				bytes="$(sysctl -n hw.memsize 2>/dev/null || echo 0)"
				case "$bytes" in
					""|*[!0-9]*) ;;
					*)
						if [ "$bytes" -gt 0 ]; then
							awk "BEGIN {printf \"%.1f GB\", $bytes/1024/1024/1024}"
							return
						fi
						;;
				esac
			fi
			;;
		Linux)
			if [ -r /proc/meminfo ]; then
				kb="$(awk '/MemTotal/ {print $2; exit}' /proc/meminfo)"
				case "$kb" in
					""|*[!0-9]*) ;;
					*)
						if [ "$kb" -gt 0 ]; then
							awk "BEGIN {printf \"%.1f GB\", $kb/1024/1024}"
							return
						fi
						;;
				esac
			fi
			;;
	esac
	printf '%s\n' "unknown"
}

disk_report() {
	path="$1"
	parent="$(dirname "$path")"
	mkdir -p "$parent"
	df -Pk "$parent" 2>/dev/null | awk 'NR==2 {printf "%.1f GB available", $4/1024/1024; found=1} END {if (!found) print "unknown"}'
}

port_available() {
	port="$1"
	if command_exists lsof; then
		if lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
			return 1
		fi
		return 0
	fi
	if command_exists nc; then
		if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
			return 1
		fi
	fi
	return 0
}

run_logged() {
	log="$1"
	shift
	say "$ $*" | tee -a "$log" >/dev/null
	if [ -t 1 ]; then
		printf '  Working'
		(
			while :; do
				sleep 2
				printf '.'
			done
		) &
		progress_pid="$!"
	else
		progress_pid=""
	fi
	if "$@" >>"$log" 2>&1; then
		status=0
	else
		status="$?"
	fi
	if [ -n "$progress_pid" ]; then
		kill "$progress_pid" >/dev/null 2>&1 || true
		wait "$progress_pid" >/dev/null 2>&1 || true
		if [ "$status" -eq 0 ]; then
			printf ' done\n'
		else
			printf ' failed\n'
		fi
	fi
	return "$status"
}

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$repo_root"
BRAND_ICON_SRC="$repo_root/frontend/public/favicon.svg"
BRAND_ICON_DST=""
setup_colors

banner "Public Beta Installer" "Local-first setup for the web app, CLI, TUI, and localhost server."
say "Welcome. Yemaka will be installed locally on this computer."
say "Protected defaults stay off during install: internet/search, cloud, connectors, embeddings, background jobs, and model downloads."
say "Press Enter to accept the recommended choices."
say ""

if [ -z "$INSTALL_TYPE" ]; then
	INSTALL_TYPE="$(ask "Choose experience: web app + CLI/TUI, or CLI/TUI only" "web")"
fi
case "$INSTALL_TYPE" in
	web|cli) ;;
	*) fail "installation type must be web or cli" ;;
esac

if [ -z "$INSTALL_HOME" ]; then
	INSTALL_HOME="$(ask "Choose Yemaka data folder" "$(default_home)")"
fi
INSTALL_HOME="$(absolute_path "$INSTALL_HOME")" || fail "could not resolve Yemaka home"
validate_install_home "$INSTALL_HOME"
if [ -z "$BIN_DIR" ]; then
	BIN_DIR="$(ask "Create the yemaka command in this folder" "$(default_bin_dir)")"
fi
BIN_DIR="$(absolute_path "$BIN_DIR")" || fail "could not resolve command shim directory"
validate_bin_dir "$BIN_DIR"

mkdir -p "$INSTALL_HOME" "$INSTALL_HOME/logs" "$INSTALL_HOME/libexec"
INSTALL_CREATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf 'Yemaka install home\ncreated_at=%s\nrepo=%s\ninstall_type=%s\nbin_dir=%s\n' "$INSTALL_CREATED_AT" "$repo_root" "$INSTALL_TYPE" "$BIN_DIR" >"$INSTALL_HOME/.yemaka-install-root"
LOG_FILE="$INSTALL_HOME/logs/install-$(date -u +%Y%m%dT%H%M%SZ).log"
: >"$LOG_FILE"

section "Install Plan"
kv "Repository" "$repo_root"
kv "Install mode" "$INSTALL_TYPE"
kv "Yemaka home" "$INSTALL_HOME"
kv "Command directory" "$BIN_DIR"
kv "Install log" "$LOG_FILE"

os_name="$(uname -s 2>/dev/null || echo unknown)"
arch_name="$(uname -m 2>/dev/null || echo unknown)"
section "System Check"
kv "System" "$os_name $arch_name"
kv "RAM" "$(ram_report)"
kv "Disk near install" "$(disk_report "$INSTALL_HOME")"

if [ "$INSTALL_TYPE" = "web" ]; then
	if port_available "$PORT"; then
		ok "Port $PORT is available."
	else
		warn "Port $PORT is already in use. You can run yemaka serve --addr 127.0.0.1:<free-port>."
	fi
fi

if command_exists ollama; then
	ok "Ollama found at $(command -v ollama)."
	if ollama list >>"$LOG_FILE" 2>&1; then
		ok "Ollama responded to model list."
	else
		warn "Ollama is installed but not responding. Start it before first chat."
	fi
else
	warn "Ollama was not found. Install Ollama and a small local model before first chat."
fi

section "Model Setup"
say "Yemaka uses Ollama models you install separately. This installer will not download models."
say "Recommended starting points:"
say "  4GB RAM: CLI/TUI low-memory only; try qwen3.5:2b-q4_K_M or similar."
say "  8GB RAM: web app + a small local model."
say "  16GB+ RAM: stronger 4B local models become more comfortable."
say "  Apple Silicon low-memory option: qwen3.5:2b-nvfp4 if available."
say ""

if confirm "Use an already-installed Ollama model now?" "no"; then
	MODEL_NAME="$(ask "Ollama model name" "")"
else
	MODEL_NAME=""
fi
if confirm "Keep low-memory friendly guidance?" "yes"; then
	LOW_MEMORY="yes"
else
	LOW_MEMORY="no"
fi

section "Protected Defaults"
ok "Internet/search remains off."
ok "Cloud fallback remains off."
ok "Connectors remain off."
ok "Embeddings/vector database remain off."
ok "No background jobs or model downloads are started."
note "You can enable optional systems later from Settings or the yemaka CLI."

INSTALL_BIN="$INSTALL_HOME/libexec/yemaka"
section "Install Files"
if [ -f "$repo_root/cmd/yemaka/main.go" ]; then
	command_exists go || fail "Go is required to build Yemaka from source. Install Go or provide a built yemaka binary."
	say "Building the local Yemaka app binary."
	note "This can take a minute on the first run while Go compiles local SQLite/database support. Build details are saved to: $LOG_FILE"
	if run_logged "$LOG_FILE" go build -o "$INSTALL_BIN" ./cmd/yemaka; then
		ok "Built app binary: $INSTALL_BIN"
	else
		fail "build failed. See the install log: $LOG_FILE"
	fi
elif command_exists yemaka; then
	say "Using existing yemaka binary from PATH."
	cp "$(command -v yemaka)" "$INSTALL_BIN"
	ok "Copied app binary: $INSTALL_BIN"
else
	fail "Cannot find source entrypoint or an existing yemaka binary."
fi
chmod +x "$INSTALL_BIN"

if [ "$INSTALL_TYPE" = "web" ]; then
	if [ ! -f "$repo_root/frontend/dist/index.html" ]; then
		if [ "$SKIP_FRONTEND_BUILD" -eq 1 ]; then
			fail "frontend/dist is missing and frontend build was skipped"
		fi
		command_exists npm || fail "npm is required because frontend/dist is missing"
		say "Building web app assets."
		note "This can take a minute. Frontend build details are saved to: $LOG_FILE"
		if run_logged "$LOG_FILE" npm --prefix frontend run build; then
			ok "Built web app assets."
		else
			fail "web build failed. See the install log: $LOG_FILE"
		fi
	fi
	mkdir -p "$INSTALL_HOME/web"
	rm -rf "$INSTALL_HOME/web/dist"
	cp -R "$repo_root/frontend/dist" "$INSTALL_HOME/web/dist"
	ok "Installed web assets: $INSTALL_HOME/web/dist"
fi

if [ -f "$BRAND_ICON_SRC" ]; then
	mkdir -p "$INSTALL_HOME/brand"
	BRAND_ICON_DST="$INSTALL_HOME/brand/favicon.svg"
	cp "$BRAND_ICON_SRC" "$BRAND_ICON_DST"
	ok "Installed brand icon: $BRAND_ICON_DST"
fi

TEMPLATE_SRC="$repo_root/packs/templates"
TEMPLATE_DST="$INSTALL_HOME/packs/templates"
[ -d "$TEMPLATE_SRC" ] || fail "built-in domain pack templates are missing: $TEMPLATE_SRC"
mkdir -p "$INSTALL_HOME/packs"
rm -rf "$TEMPLATE_DST"
cp -R "$TEMPLATE_SRC" "$TEMPLATE_DST"
ok "Installed domain pack templates: $TEMPLATE_DST"

DEFAULT_SKILLS_SRC="$repo_root/skills/default"
DEFAULT_SKILLS_DST="$INSTALL_HOME/skills/default"
[ -d "$DEFAULT_SKILLS_SRC" ] || fail "built-in default skills are missing: $DEFAULT_SKILLS_SRC"
mkdir -p "$INSTALL_HOME/skills"
rm -rf "$DEFAULT_SKILLS_DST"
cp -R "$DEFAULT_SKILLS_SRC" "$DEFAULT_SKILLS_DST"
ok "Installed default skills: $DEFAULT_SKILLS_DST"

if [ "$CREATE_SHIM" -eq 1 ]; then
	mkdir -p "$BIN_DIR"
	SHIM="$BIN_DIR/yemaka"
	cat >"$SHIM" <<EOF
#!/usr/bin/env sh
export YEMAKA_HOME="$INSTALL_HOME"
exec "$INSTALL_BIN" "\$@"
EOF
	chmod +x "$SHIM"
	ok "Command shim created: $SHIM"
	if ! printf '%s' "$PATH" | tr ':' '\n' | grep -qx "$BIN_DIR"; then
		warn "$BIN_DIR is not on PATH. Add it to your shell profile to run 'yemaka' directly."
	fi
else
	SHIM=""
fi

RECEIPT_PATH="$(write_install_receipt)"
ok "Installation receipt: $RECEIPT_PATH"

section "Verification"
say "Creating and checking the default local config..."
YEMAKA_HOME="$INSTALL_HOME" "$INSTALL_BIN" --help >>"$LOG_FILE" 2>&1
if YEMAKA_HOME="$INSTALL_HOME" "$INSTALL_BIN" doctor >>"$LOG_FILE" 2>&1; then
	ok "Doctor completed."
else
	warn "Doctor completed with warnings; see $LOG_FILE"
fi

if [ -n "$MODEL_NAME" ]; then
	say "Applying selected model roles: $MODEL_NAME"
	YEMAKA_HOME="$INSTALL_HOME" "$INSTALL_BIN" model set default "$MODEL_NAME" >>"$LOG_FILE" 2>&1 || warn "Could not set default model"
	YEMAKA_HOME="$INSTALL_HOME" "$INSTALL_BIN" model set low_memory "$MODEL_NAME" >>"$LOG_FILE" 2>&1 || warn "Could not set low-memory model"
fi

say ""
say "${BRAND}${BOLD}Installation complete.${RESET}"
say "Try:"
if [ "$CREATE_SHIM" -eq 1 ]; then
	say "  $BIN_DIR/yemaka --help"
	say "  $BIN_DIR/yemaka doctor"
	if [ "$INSTALL_TYPE" = "web" ]; then
		say "  $BIN_DIR/yemaka serve"
	fi
	say "  $BIN_DIR/yemaka tui"
else
	say "  YEMAKA_HOME=\"$INSTALL_HOME\" \"$INSTALL_BIN\" --help"
	say "  YEMAKA_HOME=\"$INSTALL_HOME\" \"$INSTALL_BIN\" doctor"
fi
say "Uninstall later with: scripts/install/uninstall.sh"
say "The uninstaller will prefill this install from the receipt above."

case "$(printf '%s' "$LOW_MEMORY" | tr '[:upper:]' '[:lower:]')" in
	yes|y|true|1)
		say "Low-memory guidance kept active. Choose an installed small model with: yemaka model set low_memory <model>"
		;;
esac

say "No cloud, internet/search, connectors, embeddings, background jobs, or model downloads were enabled by this installer."
