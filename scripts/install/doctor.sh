#!/usr/bin/env sh
set -eu

PORT="7727"
INSTALL_HOME="${YEMAKA_HOME:-}"
YEMAKA_BIN="${YEMAKA_BIN:-}"

usage() {
	cat <<'EOF'
Usage: scripts/install/doctor.sh [options]

Options:
  --home PATH       Yemaka data/runtime directory to check.
  --bin PATH        Yemaka binary or command shim to check.
  --port PORT       Port to check for yemaka serve (default 7727).
  -h, --help        Show this help.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
		--home)
			shift
			INSTALL_HOME="${1:-}"
			;;
		--bin)
			shift
			YEMAKA_BIN="${1:-}"
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

expand_path() {
	case "$1" in
		~) printf '%s\n' "$HOME" ;;
		~/*) printf '%s/%s\n' "$HOME" "${1#~/}" ;;
		*) printf '%s\n' "$1" ;;
	esac
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

command_exists() {
	command -v "$1" >/dev/null 2>&1
}

ram_report() {
	case "$(uname -s 2>/dev/null || echo unknown)" in
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
	if [ ! -d "$parent" ]; then
		parent="$HOME"
	fi
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

if [ -z "$INSTALL_HOME" ]; then
	INSTALL_HOME="$(default_home)"
fi
INSTALL_HOME="$(expand_path "$INSTALL_HOME")"

if [ -z "$YEMAKA_BIN" ]; then
	if command_exists yemaka; then
		YEMAKA_BIN="$(command -v yemaka)"
	elif [ -x "$INSTALL_HOME/libexec/yemaka" ]; then
		YEMAKA_BIN="$INSTALL_HOME/libexec/yemaka"
	elif [ -x "$INSTALL_HOME/bin/yemaka" ]; then
		YEMAKA_BIN="$INSTALL_HOME/bin/yemaka"
	else
		YEMAKA_BIN="yemaka"
	fi
fi

say "Yemaka install doctor"
say "os: $(uname -s 2>/dev/null || echo unknown)"
say "arch: $(uname -m 2>/dev/null || echo unknown)"
say "ram: $(ram_report)"
say "disk: $(disk_report "$INSTALL_HOME")"
say "path_contains_binary: $(command_exists yemaka && echo true || echo false)"
say "yemaka_home: $INSTALL_HOME"
say "binary: $YEMAKA_BIN"

if [ -d "$INSTALL_HOME" ]; then
	say "home_exists: true"
else
	say "home_exists: false"
fi

if [ -f "$INSTALL_HOME/config.yaml" ]; then
	say "config_exists: true"
else
	say "config_exists: false"
fi

if [ -f "$INSTALL_HOME/web/dist/index.html" ]; then
	say "web_build: installed"
elif [ -f "frontend/dist/index.html" ]; then
	say "web_build: development"
else
	say "web_build: missing"
fi

if port_available "$PORT"; then
	say "port_$PORT: available"
else
	say "port_$PORT: in_use"
fi

if command_exists ollama; then
	say "ollama_installed: true"
	if ollama list >/dev/null 2>&1; then
		say "ollama_running: true"
	else
		say "ollama_running: false"
	fi
else
	say "ollama_installed: false"
	say "ollama_running: false"
fi

if [ -x "$YEMAKA_BIN" ] || command_exists "$YEMAKA_BIN"; then
	say ""
	say "yemaka doctor:"
	YEMAKA_HOME="$INSTALL_HOME" "$YEMAKA_BIN" doctor || true
else
	say "yemaka_binary_ready: false"
fi
