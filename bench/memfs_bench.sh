#!/usr/bin/env bash
#
# memfs_bench.sh -- compare FUSE protocol overhead by benchmarking the memfs
# example mounted under different cgofuse build variants (e.g. FUSE2 vs FUSE3).
#
# memfs keeps all data in RAM, so the numbers reflect FUSE transport + cgo
# overhead rather than any backing store. Run from the repository root:
#
#   bench/memfs_bench.sh                 # builds + benchmarks fuse2 and fuse3
#   ITERS=5 SIZE=1G bench/memfs_bench.sh # override iterations / file size
#   bench/memfs_bench.sh -o io_uring     # pass extra mount options to every run
#
# Requires: go, fio, fusermount, a kernel with /dev/fuse. No root needed.
#
set -euo pipefail

# Default size is modest because FUSE2 without big_writes caps writes at one
# page (4K), so its sequential write is very slow and otherwise dominates the
# run. Override SIZE for a heavier comparison.
ITERS="${ITERS:-2}"
SIZE="${SIZE:-64M}"
OUT="${OUT:-$(mktemp -d)}"
EXTRA_OPTS=("$@")

command -v fio >/dev/null || { echo "fio not found"; exit 1; }
[ -e /dev/fuse ] || { echo "/dev/fuse missing"; exit 1; }

# variant label -> go build tags
VARIANTS=("fuse2:" "fuse3:fuse3")

bin_for() { echo "$OUT/memfs_$1"; }

build() {
	local label="$1" tags="$2" args=()
	[ -n "$tags" ] && args=(-tags="$tags")
	go build "${args[@]}" -o "$(bin_for "$label")" ./examples/memfs
}

# run one fio job against a directory, echo "<MB/s> <IOPS>"
fio_job() {
	local dir="$1" name="$2" rw="$3" bs="$4"
	fio --name="$name" --directory="$dir" --rw="$rw" --bs="$bs" \
		--size="$SIZE" --ioengine=psync --direct=0 --fsync=0 \
		--numjobs=1 --group_reporting --minimal 2>/dev/null \
	| awk -F';' '{
		# minimal format: read bw KB/s = $7, read iops = $8;
		# write bw KB/s = $48, write iops = $49
		rbw=$7; riops=$8; wbw=$48; wiops=$49;
		bw=(rbw>0?rbw:wbw); iops=(riops>0?riops:wiops);
		printf "%.0f %.0f\n", bw/1024, iops
	}'
}

bench_variant() {
	local label="$1" tags="$2"
	local bin mnt
	bin="$(bin_for "$label")"
	mnt="$OUT/mnt_$label"
	mkdir -p "$mnt"

	"$bin" "${EXTRA_OPTS[@]}" "$mnt" &
	local pid=$!
	# wait for the mount to appear
	for _ in $(seq 1 50); do
		mountpoint -q "$mnt" && break; sleep 0.1
	done
	mountpoint -q "$mnt" || { echo "mount failed for $label"; kill "$pid" 2>/dev/null || true; return 1; }

	local sw=0 sr=0 srr=0 out
	for _ in $(seq 1 "$ITERS"); do
		out=$(fio_job "$mnt" seqwrite write 1M);    sw=$((sw + ${out%% *}))
		out=$(fio_job "$mnt" seqread  read  1M);    sr=$((sr + ${out%% *}))
		out=$(fio_job "$mnt" randread randread 4k); srr=$((srr + ${out##* }))
		rm -f "$mnt"/*
	done

	fusermount -u "$mnt" >/dev/null 2>&1 || true
	wait "$pid" 2>/dev/null || true

	printf "%-7s %12d %12d %14d\n" \
		"$label" "$((sw/ITERS))" "$((sr/ITERS))" "$((srr/ITERS))"
}

echo "memfs benchmark: iters=$ITERS size=$SIZE opts='${EXTRA_OPTS[*]}' out=$OUT"
echo "kernel $(uname -r), $(nproc) cpus"
printf "%-7s %12s %12s %14s\n" "variant" "seqwr MB/s" "seqrd MB/s" "randrd 4k IOPS"
for v in "${VARIANTS[@]}"; do
	label="${v%%:*}"; tags="${v#*:}"
	build "$label" "$tags"
	bench_variant "$label" "$tags"
done
