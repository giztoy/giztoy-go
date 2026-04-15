#!/usr/bin/env bash
set -euo pipefail

LOSS_PCT=0
DELAY_MS=0
BENCH_FILTER='BenchmarkNet_'
BENCHTIME='3s'
COUNT=1
SCALE='smoke' # smoke|full
GO_TEST_TAGS=''
MANUAL_MODE=0
BASE_PORT=41000
USE_SYSTEM_NETEM=1
INTERNAL_LOSS_RATES='0'

PROFILE='' # clean|wifi|mobile|bad|all
PUBLISH_RESULT=0
RESULT_FILE=''
RESULT_DIR='pkg/giznet/internal/benchmark'

LINUX_IFACE='lo'
MAC_PIPE_ID=41

PF_WAS_ENABLED=0
TMP_PF_RULES=''

timestamp=''
out_dir=''
report_file=''

usage() {
	cat <<'EOF'
Usage: ./pkg/giznet/internal/benchmark/run.sh [options]

Options:
  --profile <name>             Fixed profile: clean|wifi|mobile|bad|all
  --scale <mode>               Matrix scale: smoke|full (default: smoke)
  --loss <pct>                 System-level packet loss percent (default: 0)
  --delay <ms>                 System-level one-way delay in ms (default: 0)
  --bench <regex>              go test -bench filter (default: BenchmarkNet_)
  --benchtime <dur>            go test -benchtime (default: 3s)
  --count <n>                  go test -count (default: 1)
  --manual                     Enable manual mode (uses -tags manual)
  --base-port <port>           UDP base port (A=base, B=base+1, default: 41000)
  --linux-iface <iface>        Linux tc interface (default: lo)
  --internal-loss <list>       In-process loss list, e.g. 0,0.01,0.05 (default: 0)
  --no-system-netem            Skip tc/pfctl netem; only use in-process loss model
  --publish-result             Publish report to pkg/giznet/internal/benchmark/SMOKE_RESULT.md or FULL_RESULT.md
  --result-file <path>         Publish target path (overrides default result file)
  --update-readme              Deprecated alias of --publish-result
  --readme <path>              Deprecated alias of --result-file
  -h, --help                   Show this help

Fixed profile defaults:
  clean   -> loss=0%,  delay=0ms,   internal-loss=0
  wifi    -> loss=1%,  delay=20ms,  internal-loss=0.01
  mobile  -> loss=5%,  delay=60ms,  internal-loss=0.05
  bad     -> loss=10%, delay=120ms, internal-loss=0.10

Examples:
  ./pkg/giznet/internal/benchmark/run.sh --profile clean --scale smoke --no-system-netem
  ./pkg/giznet/internal/benchmark/run.sh --profile all --scale full --manual --publish-result
  ./pkg/giznet/internal/benchmark/run.sh --profile all --scale full --manual --bench 'BenchmarkNet_KCPOverNoise_'
EOF
}

require_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "[benchmark] missing command: $1" >&2
		exit 1
	fi
}

cleanup_linux_netem() {
	sudo tc qdisc del dev "$LINUX_IFACE" root >/dev/null 2>&1 || true
}

apply_linux_netem() {
	local loss="$1"
	local delay="$2"
	require_cmd tc
	echo "[benchmark] linux netem: iface=${LINUX_IFACE}, loss=${loss}%, delay=${delay}ms"
	cleanup_linux_netem
	sudo tc qdisc replace dev "$LINUX_IFACE" root netem loss "${loss}%" delay "${delay}ms"
}

cleanup_macos_netem() {
	sudo dnctl -q pipe "$MAC_PIPE_ID" delete >/dev/null 2>&1 || true
	if [[ -n "$TMP_PF_RULES" && -f "$TMP_PF_RULES" ]]; then
		rm -f "$TMP_PF_RULES"
	fi
	sudo pfctl -f /etc/pf.conf >/dev/null 2>&1 || true
	if [[ "$PF_WAS_ENABLED" -eq 0 ]]; then
		sudo pfctl -d >/dev/null 2>&1 || true
	fi
}

apply_macos_netem() {
	local loss="$1"
	local delay="$2"
	require_cmd pfctl
	require_cmd dnctl

	local plr
	plr=$(awk "BEGIN { printf \"%.6f\", ${loss}/100.0 }")

	local pf_info
	pf_info="$(sudo pfctl -s info 2>/dev/null || true)"
	case "$pf_info" in
	*"Status: Enabled"*) PF_WAS_ENABLED=1 ;;
	*) PF_WAS_ENABLED=0 ;;
	esac

	TMP_PF_RULES="$(mktemp -t gizclaw-bench-pf.XXXXXX)"
	cat >"$TMP_PF_RULES" <<EOF
dummynet in quick proto udp from any to any port {${BASE_PORT},$((BASE_PORT + 1))} pipe ${MAC_PIPE_ID}
dummynet out quick proto udp from any to any port {${BASE_PORT},$((BASE_PORT + 1))} pipe ${MAC_PIPE_ID}
pass quick all
EOF

	echo "[benchmark] macOS dummynet: pipe=${MAC_PIPE_ID}, plr=${plr}, delay=${delay}ms"
	sudo dnctl pipe "$MAC_PIPE_ID" config plr "$plr" delay "${delay}ms"
	sudo pfctl -E >/dev/null 2>&1 || true
	sudo pfctl -f "$TMP_PF_RULES"
}

cleanup() {
	if [[ "$USE_SYSTEM_NETEM" -eq 1 ]]; then
		case "$(uname -s)" in
		Linux) cleanup_linux_netem ;;
		Darwin) cleanup_macos_netem ;;
		esac
	fi
}

profile_values() {
	local name="$1"
	case "$name" in
	clean) echo "0 0 0" ;;
	wifi) echo "1 20 0.01" ;;
	mobile) echo "5 60 0.05" ;;
	bad) echo "10 120 0.10" ;;
	*)
		echo "[benchmark] unsupported profile: ${name}" >&2
		exit 1
		;;
	esac
}

to_pct_label() {
	local v="$1"
	awk -v x="$v" 'BEGIN {
		if (x <= 1.0) {
			printf "%.2f", x * 100.0
		} else {
			printf "%.2f", x
		}
	}'
}

run_one() {
	local profile_name="$1"
	local loss="$2"
	local delay="$3"
	local internal_loss="$4"

	local out_file="${out_dir}/${profile_name}.txt"
	echo "[benchmark] ===== profile=${profile_name} loss=${loss}% delay=${delay}ms internal=${internal_loss} scale=${SCALE} ====="

	if [[ "$USE_SYSTEM_NETEM" -eq 1 ]]; then
		case "$(uname -s)" in
		Linux)
			apply_linux_netem "$loss" "$delay"
			;;
		Darwin)
			apply_macos_netem "$loss" "$delay"
			;;
		*)
			echo "[benchmark] unsupported OS: $(uname -s)" >&2
			exit 1
			;;
		esac
	fi

	{
		echo "# profile=${profile_name}"
		echo "# scale=${SCALE}"
		echo "# manual=${MANUAL_MODE} tags=${GO_TEST_TAGS:-<none>}"
		echo "# system-netem=${USE_SYSTEM_NETEM} loss=${loss}% delay=${delay}ms"
		echo "# internal-loss=${internal_loss} base-port=${BASE_PORT}"
		echo "# bench=${BENCH_FILTER} benchtime=${BENCHTIME} count=${COUNT}"
	} >"$out_file"

	local -a cmd
	cmd=(go test ./pkg/giznet/internal/benchmark -run '^$' -bench "$BENCH_FILTER" -benchmem -count "$COUNT" -benchtime "$BENCHTIME")
	if [[ -n "$GO_TEST_TAGS" ]]; then
		cmd+=(-tags "$GO_TEST_TAGS")
	fi

	BENCH_NET_UDP_BASE_PORT="$BASE_PORT" \
		BENCH_NET_SCALE="$SCALE" \
		BENCH_NET_MANUAL="$MANUAL_MODE" \
		BENCH_NET_LOSS_RATES="$internal_loss" \
		"${cmd[@]}" | tee -a "$out_file"

	# In profile-all mode, ensure old netem state is cleared before next profile.
	if [[ "$USE_SYSTEM_NETEM" -eq 1 ]]; then
		cleanup
	fi
}

collect_machine_info() {
	echo "- Run timestamp: $(date '+%Y-%m-%d %H:%M:%S %z')"
	echo "- Go version: $(go version)"
	echo "- OS: $(uname -s) $(uname -r)"
	echo "- Arch: $(uname -m)"

	case "$(uname -s)" in
	Darwin)
		echo "- macOS version: $(sw_vers -productVersion 2>/dev/null || echo unknown)"
		echo "- Model: $(sysctl -n hw.model 2>/dev/null || echo unknown)"
		echo "- CPU: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo 'Apple Silicon')"
		echo "- Logical cores: $(sysctl -n hw.logicalcpu 2>/dev/null || echo unknown)"
		echo "- Memory bytes: $(sysctl -n hw.memsize 2>/dev/null || echo unknown)"
		;;
	Linux)
		echo "- CPU: $(lscpu 2>/dev/null | awk -F: '/Model name/ {gsub(/^ +/,"",$2); print $2; exit}' || echo unknown)"
		echo "- Logical cores: $(nproc 2>/dev/null || echo unknown)"
		echo "- Memory: $(grep -m1 MemTotal /proc/meminfo 2>/dev/null | awk '{print $2 " " $3}' || echo unknown)"
		;;
	esac
}

summary_table_for_profile() {
	local file="$1"
	awk '
BEGIN {
	print "| Key | MB/s | ns/op | drop% | delivery% | allocs/op | Details |"
	print "|---|---:|---:|---:|---:|---:|---|"
	idx = 0
}
/^BenchmarkNet_/ {
	idx++
	name = $1
	nsop = "-"
	mbps = "-"
	drop = "-"
	delivery = "-"
	allocs = "-"
	for (i = 2; i <= NF; i++) {
		if ($i == "ns/op") nsop = $(i-1)
		else if ($i == "MB/s") mbps = $(i-1)
		else if ($i == "drop_pct") drop = $(i-1)
		else if ($i == "delivery_pct") delivery = $(i-1)
		else if ($i == "allocs/op") allocs = $(i-1)
	}
	key = sprintf("R%02d", idx)
	detail = name
	sub(/-[0-9]+$/, "", detail)
	printf("| %s | %s | %s | %s | %s | %s | `%s` |\n", key, mbps, nsop, drop, delivery, allocs, detail)
}
' "$file"
}

generate_report() {
	report_file="${out_dir}/report.md"
	{
		echo "# Net Benchmark Report (${timestamp})"
		echo
		echo "## 1) Test Conditions"
		echo
		echo "- Script: \`./pkg/giznet/internal/benchmark/run.sh\`"
		echo "- Scale: \`${SCALE}\`"
		echo "- Bench filter: \`${BENCH_FILTER}\`"
		echo "- Benchtime: \`${BENCHTIME}\`"
		echo "- Count: \`${COUNT}\`"
		echo "- Manual mode: \`${MANUAL_MODE}\`"
		echo "- Go tags: \`${GO_TEST_TAGS:-<none>}\`"
		echo "- System-level netem: \`${USE_SYSTEM_NETEM}\` (1=enabled, 0=disabled)"
		if [[ "$USE_SYSTEM_NETEM" -eq 0 ]]; then
			echo "- NOTE: --no-system-netem is enabled; profile loss/delay are metadata only, and only in-process internal-loss is effective."
		fi
		echo "- UDP base ports: \`${BASE_PORT}\` and \`$((BASE_PORT + 1))\`"
		echo
		echo "| Profile | loss | delay | internal-loss |"
		echo "|---|---:|---:|---:|"
		for p in "${RAN_PROFILES[@]}"; do
			if [[ "$p" == "custom" ]]; then
				pl="$LOSS_PCT"
				pd="$DELAY_MS"
				pi="$INTERNAL_LOSS_RATES"
			else
				read -r pl pd pi <<<"$(profile_values "$p")"
			fi
			echo "| ${p} | ${pl}% | ${pd}ms | $(to_pct_label "$pi")% |"
		done
		echo
		echo "## 2) Machine Info"
		echo
		collect_machine_info
		echo
		echo "## 3) Benchmark Summary"
		echo
		echo "Key column intentionally uses short IDs (Rxx). Full benchmark case is in the last **Details** column."
		echo
		for p in "${RAN_PROFILES[@]}"; do
			echo "### Profile: ${p}"
			echo
			summary_table_for_profile "${out_dir}/${p}.txt"
			echo
		done
		echo "## 4) Raw Outputs"
		echo
		for p in "${RAN_PROFILES[@]}"; do
			echo "### Profile: ${p}"
			echo
			echo "<details>"
			echo "<summary>Expand raw output: ${p}</summary>"
			echo
			echo '```text'
			cat "${out_dir}/${p}.txt"
			echo '```'
			echo "</details>"
			echo
		done
	} >"$report_file"
}

publish_result() {
	local target
	if [[ -n "$RESULT_FILE" ]]; then
		target="$RESULT_FILE"
	else
		if [[ "$SCALE" == "smoke" ]]; then
			target="${RESULT_DIR}/SMOKE_RESULT.md"
		else
			target="${RESULT_DIR}/FULL_RESULT.md"
		fi
	fi

	cp "$report_file" "$target"
	echo "[benchmark] published: ${target}"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--profile)
		PROFILE="$2"
		shift 2
		;;
	--scale)
		SCALE="$2"
		shift 2
		;;
	--loss)
		LOSS_PCT="$2"
		shift 2
		;;
	--delay)
		DELAY_MS="$2"
		shift 2
		;;
	--bench)
		BENCH_FILTER="$2"
		shift 2
		;;
	--benchtime)
		BENCHTIME="$2"
		shift 2
		;;
	--count)
		COUNT="$2"
		shift 2
		;;
	--manual)
		MANUAL_MODE=1
		GO_TEST_TAGS="manual"
		shift
		;;
	--base-port)
		BASE_PORT="$2"
		shift 2
		;;
	--linux-iface)
		LINUX_IFACE="$2"
		shift 2
		;;
	--internal-loss)
		INTERNAL_LOSS_RATES="$2"
		shift 2
		;;
	--no-system-netem)
		USE_SYSTEM_NETEM=0
		shift
		;;
	--publish-result)
		PUBLISH_RESULT=1
		shift
		;;
	--result-file)
		RESULT_FILE="$2"
		shift 2
		;;
	--update-readme)
		echo "[benchmark] warning: --update-readme is deprecated, using --publish-result" >&2
		PUBLISH_RESULT=1
		shift
		;;
	--readme)
		echo "[benchmark] warning: --readme is deprecated, using --result-file" >&2
		RESULT_FILE="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "[benchmark] unknown option: $1" >&2
		usage
		exit 1
		;;
	esac
done

case "$SCALE" in
smoke | full) ;;
*)
	echo "[benchmark] invalid --scale: ${SCALE} (use smoke|full)" >&2
	exit 1
	;;
esac

if [[ "$SCALE" != "smoke" && "$MANUAL_MODE" -ne 1 ]]; then
	echo "[benchmark] scale=${SCALE} requires --manual" >&2
	exit 1
fi

require_cmd go

timestamp="$(date +%Y%m%d-%H%M%S)"
out_dir="pkg/giznet/internal/benchmark/results/${timestamp}"
mkdir -p "$out_dir"

echo "[benchmark] output dir: ${out_dir}"

if [[ -n "$PROFILE" && "$USE_SYSTEM_NETEM" -eq 0 ]]; then
	echo "[benchmark] warning: --profile with --no-system-netem applies internal-loss only; system loss/delay are not injected"
fi

declare -a RAN_PROFILES=()

trap cleanup EXIT

if [[ -n "$PROFILE" ]]; then
	case "$PROFILE" in
	all)
		for p in clean wifi mobile bad; do
			read -r lp dp il <<<"$(profile_values "$p")"
			RAN_PROFILES+=("$p")
			run_one "$p" "$lp" "$dp" "$il"
		done
		;;
	clean | wifi | mobile | bad)
		read -r lp dp il <<<"$(profile_values "$PROFILE")"
		RAN_PROFILES+=("$PROFILE")
		run_one "$PROFILE" "$lp" "$dp" "$il"
		;;
	*)
		echo "[benchmark] unsupported profile: ${PROFILE}" >&2
		exit 1
		;;
	esac
else
	RAN_PROFILES+=("custom")
	run_one "custom" "$LOSS_PCT" "$DELAY_MS" "$INTERNAL_LOSS_RATES"
fi

generate_report
echo "[benchmark] report: ${report_file}"

if [[ "$PUBLISH_RESULT" -eq 1 ]]; then
	publish_result
fi

echo "[benchmark] done"
