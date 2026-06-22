# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`github.com/winfsp/cgofuse` — a cross-platform FUSE library for Go. User-mode
file systems implement `fuse.FileSystemInterface` (or embed `fuse.FileSystemBase`
for defaults) and mount via `fuse.NewFileSystemHost(...).Mount(...)`. Single Go
module, `go 1.26`, no third-party dependencies. The only importable package is
`./fuse`; `./examples/...` are runnable demo file systems.

This checkout lives inside the `~/git/gMountie` org container as a dependency of
gMountie. It tracks upstream `winfsp/cgofuse`; treat it as vendored upstream
unless you are deliberately forking — keep changes minimal and upstream-shaped.

## Build & test

There is no Makefile or task runner; use `go` directly. Most work happens in the
`./fuse` package.

```bash
# Build everything, FUSE2 (default), into the working dir
go build -v -o . ./...

# Build with FUSE3 (Linux & FreeBSD only)
go build -tags=fuse3 -v -o . ./...

# memfs has an alternate FUSE3-style implementation behind a build tag
go build -tags=memfs3 -o memfs3 ./examples/memfs

# Unit tests (the only `go test`-runnable suite)
go test -v -count=1 ./fuse
go test -tags=fuse3 -v -count=1 ./fuse     # FUSE3 variant
go test -run TestOptParse ./fuse           # single test
```

Build variant is selected by environment/tags, not flags in code:
- **cgo** (default on Linux/macOS/BSD/Windows): requires a C toolchain + FUSE
  headers. Linux needs `libfuse-dev` (FUSE2) and/or `libfuse3-dev` (FUSE3); on
  Windows set `CPATH=C:\Program Files (x86)\WinFsp\inc\fuse`.
- **nocgo** (Windows only): `CGO_ENABLED=0`. Pure-Go path that talks to WinFsp
  directly via syscalls.

The unit tests (`./fuse`) are pure Go and run anywhere. Real file-system
conformance — `secfs.test`/`fstest` and `fsx` against the example mounts — only
runs in CI (`.github/workflows/test.yml`), because it needs `/dev/fuse`, root,
and an actual mount. Don't expect to reproduce those locally without a FUSE host.

## Architecture

Two layers, each split across files by build constraint. **When changing
behavior, the same change usually has to land in three places** (cgo, nocgo, and
sometimes the shared dispatch).

**Interface layer (what a file system implements):**
- `fsop.go` — shared, no build tags. Defines `FileSystemInterface`, the
  `FileSystemBase` default impl, value types (`Stat_t`, `Statfs_t`, `Timespec`,
  `FileInfo_t`, `Lock_t`), the `Error` errno type, and the optional capability
  interfaces a FS may additionally implement: `FileSystemOpenEx`,
  `FileSystemGetpath`, `FileSystemChflags`, `FileSystemSetcrtime`,
  `FileSystemSetchgtime`, and the FUSE3-only `FileSystemChmod3` / `Chown3` /
  `Utimens3` / `Rename3`. The host type-asserts for these at mount time.
- `fsop_cgo.go` (`//go:build cgo`) and `fsop_nocgo_windows.go`
  (`//go:build !cgo && windows`) — provide the platform errno constants
  (`E*`) and flag values referenced by `fsop.go`.

**Host layer (the bridge to the native FUSE engine):**
- `host.go` — shared. `FileSystemHost`, `NewFileSystemHost`, the `SetCap*` /
  `SetDirectIO` / `SetUseIno` configuration setters, `Mount` / `Unmount` /
  `Notify`, `Getcontext`, and `OptParse` (a getopt-style `-o` option parser).
  Crucially it holds the `host*` callback functions (e.g. `hostGetattr`,
  `hostRead`, `hostReaddir`) that translate a native FUSE call into a Go
  `FileSystemInterface` method call and marshal the structs back. These are the
  hot path.
- `host_cgo.go` (`//go:build cgo`) — the cgo bridge. A large `import "C"`
  preamble sets per-platform `CFLAGS` selecting `FUSE_USE_VERSION` 28 (FUSE2) or
  39 (FUSE3). **It does not link libfuse at build time** — it `dlopen`s the FUSE
  library at runtime (hence `LDFLAGS: -ldl`), so the binary builds without the
  shared library present and picks it up when mounting.
- `host_nocgo_windows.go` (`//go:build !cgo && windows`) — a pure-Go
  reimplementation of the same bridge that loads and calls the WinFsp DLL via
  `syscall`, no cgo.

Adding or changing an operation means: define/adjust it on `FileSystemInterface`
+ `FileSystemBase` in `fsop.go`, then wire the `host*` dispatch in `host.go`,
then make sure both `host_cgo.go` and `host_nocgo_windows.go` invoke it. The C
struct layouts in `host_cgo.go` and the hand-rolled struct offsets in
`host_nocgo_windows.go` must agree with the native FUSE/WinFsp ABI.

## Examples (`./examples`)

- `hellofs` — minimal read-only FS; all platforms.
- `memfs` — in-memory FS; all platforms. `memfs.go` is the FUSE2-style impl;
  `memfs3.go` (tag `memfs3`) demonstrates the FUSE3-only handle-carrying
  methods.
- `passthrough` — proxies to the underlying FS; per-platform `port_*.go` files
  supply the OS-specific syscalls. All platforms except Windows.
- `notifyfs` — demonstrates `Notify`; Windows only.
- `shared/trace.go` — a tracing helper used by the examples.

## Conventions

- Operations return a negative errno `int` (e.g. `-fuse.ENOENT`); a successful
  op returns `0` or a non-negative byte count. This mirrors libfuse, not Go
  error idiom.
- Keep the cgo and nocgo-Windows host implementations behaviorally in lockstep.
- New optional capabilities are added as small single-method interfaces in
  `fsop.go` that the host detects by type assertion, rather than by growing the
  core `FileSystemInterface`.
