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
			if [ -d /opt/homebrew/bin ] && [ -w /opt/homebrew/bin ]; then
				printf '%s\n' "/opt/homebrew/bin"
			elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
				printf '%s\n' "/usr/local/bin"
			else
				printf '%s\n' "$HOME/.local/bin"
			fi
			;;
		Linux)
			printf '%s\n' "$HOME/.local/bin"
			;;
		*)
			printf '%s\n' "$HOME/.local/bin"
			;;
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
	"$@" >>"$log" 2>&1
}

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$repo_root"

if [ -z "$INSTALL_TYPE" ]; then
	INSTALL_TYPE="$(ask "Choose installation type: web (Web + CLI/TUI) or cli (CLI/TUI only)" "web")"
fi
case "$INSTALL_TYPE" in
	web|cli) ;;
	*) fail "installation type must be web or cli" ;;
esac

if [ -z "$INSTALL_HOME" ]; then
	INSTALL_HOME="$(ask "Choose Yemaka data directory" "$(default_home)")"
fi
INSTALL_HOME="$(absolute_path "$INSTALL_HOME")" || fail "could not resolve Yemaka home"
validate_install_home "$INSTALL_HOME"
if [ -z "$BIN_DIR" ]; then
	BIN_DIR="$(ask "Choose command shim directory" "$(default_bin_dir)")"
fi
BIN_DIR="$(absolute_path "$BIN_DIR")" || fail "could not resolve command shim directory"
validate_bin_dir "$BIN_DIR"

mkdir -p "$INSTALL_HOME" "$INSTALL_HOME/logs" "$INSTALL_HOME/libexec"
printf 'Yemaka install home\ncreated_at=%s\nrepo=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$repo_root" >"$INSTALL_HOME/.yemaka-install-root"
LOG_FILE="$INSTALL_HOME/logs/install-$(date -u +%Y%m%dT%H%M%SZ).log"
: >"$LOG_FILE"

say "Yemaka public-beta installer"
say "Repository: $repo_root"
say "Install type: $INSTALL_TYPE"
say "Yemaka home: $INSTALL_HOME"
say "Command shim directory: $BIN_DIR"
say "Log: $LOG_FILE"
say ""

os_name="$(uname -s 2>/dev/null || echo unknown)"
arch_name="$(uname -m 2>/dev/null || echo unknown)"
say "System: $os_name $arch_name"
say "RAM: $(ram_report)"
say "Disk near install home: $(disk_report "$INSTALL_HOME")"

if [ "$INSTALL_TYPE" = "web" ]; then
	if port_available "$PORT"; then
		say "Port $PORT: available"
	else
		warn "Port $PORT is already in use. You can run yemaka serve --addr 127.0.0.1:<free-port>."
	fi
fi

if command_exists ollama; then
	say "Ollama: found ($(command -v ollama))"
	if ollama list >>"$LOG_FILE" 2>&1; then
		say "Ollama model list: ok"
	else
		warn "Ollama is installed but not responding. Start it before first chat."
	fi
else
	warn "Ollama was not found. Install Ollama and a small local model before first chat."
fi

say ""
say "Model guidance:"
say "  4GB RAM: CLI/TUI low-memory only; try qwen3.5:2b-q4_K_M or similar."
say "  8GB RAM: recommended minimum for web + a small local model."
say "  16GB+ RAM: stronger 4B local models become more comfortable."
say "  macOS Apple Silicon low-memory option: qwen3.5:2b-nvfp4 if available."
say ""

if confirm "Do you want to configure Ollama/model settings now?" "no"; then
	MODEL_NAME="$(ask "Installed model name to use for default and low-memory roles" "")"
else
	MODEL_NAME=""
fi
LOW_MEMORY="$(ask "Do you want to use low-memory mode guidance?" "yes")"
DISABLED_DEFAULTS="$(ask "Keep internet/search/cloud/connectors/embeddings disabled by default?" "yes")"
case "$(printf '%s' "$DISABLED_DEFAULTS" | tr '[:upper:]' '[:lower:]')" in
	yes|y|true|1) ;;
	*) warn "Installer will still preserve safe disabled defaults. Enable optional systems later from settings/CLI." ;;
esac

INSTALL_BIN="$INSTALL_HOME/libexec/yemaka"
if [ -f "$repo_root/cmd/yemaka/main.go" ]; then
	command_exists go || fail "Go is required to build Yemaka from source. Install Go or provide a built yemaka binary."
	say "Building Yemaka CLI/TUI/server binary..."
	run_logged "$LOG_FILE" go build -o "$INSTALL_BIN" ./cmd/yemaka
elif command_exists yemaka; then
	say "Using existing yemaka binary from PATH."
	cp "$(command -v yemaka)" "$INSTALL_BIN"
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
		say "Building web frontend..."
		run_logged "$LOG_FILE" npm --prefix frontend run build
	fi
	mkdir -p "$INSTALL_HOME/web"
	rm -rf "$INSTALL_HOME/web/dist"
	cp -R "$repo_root/frontend/dist" "$INSTALL_HOME/web/dist"
	say "Installed web assets: $INSTALL_HOME/web/dist"
fi

if [ "$CREATE_SHIM" -eq 1 ]; then
	mkdir -p "$BIN_DIR"
	SHIM="$BIN_DIR/yemaka"
	cat >"$SHIM" <<EOF
#!/usr/bin/env sh
export YEMAKA_HOME="$INSTALL_HOME"
exec "$INSTALL_BIN" "\$@"
EOF
	chmod +x "$SHIM"
	say "Command shim created: $SHIM"
	if ! printf '%s' "$PATH" | tr ':' '\n' | grep -qx "$BIN_DIR"; then
		warn "$BIN_DIR is not on PATH. Add it to your shell profile to run 'yemaka' directly."
	fi
fi

say "Creating/verifying default config..."
YEMAKA_HOME="$INSTALL_HOME" "$INSTALL_BIN" --help >>"$LOG_FILE" 2>&1
if YEMAKA_HOME="$INSTALL_HOME" "$INSTALL_BIN" doctor >>"$LOG_FILE" 2>&1; then
	say "Doctor: completed"
else
	warn "Doctor completed with warnings; see $LOG_FILE"
fi

if [ -n "$MODEL_NAME" ]; then
	say "Applying selected model roles: $MODEL_NAME"
	YEMAKA_HOME="$INSTALL_HOME" "$INSTALL_BIN" model set default "$MODEL_NAME" >>"$LOG_FILE" 2>&1 || warn "Could not set default model"
	YEMAKA_HOME="$INSTALL_HOME" "$INSTALL_BIN" model set low_memory "$MODEL_NAME" >>"$LOG_FILE" 2>&1 || warn "Could not set low-memory model"
fi

say ""
say "Installed. Try:"
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

case "$(printf '%s' "$LOW_MEMORY" | tr '[:upper:]' '[:lower:]')" in
	yes|y|true|1)
		say "Low-memory guidance kept active. Choose an installed small model with: yemaka model set low_memory <model>"
		;;
esac

say "No cloud, internet/search, connectors, embeddings, background jobs, or model downloads were enabled by this installer."
