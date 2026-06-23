/*
 * host_ops_test.go
 *
 * Tests for the optional-interface (hostOps) registry and its dispatch.
 */

package fuse

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// resolveTestFS implements a subset of the optional interfaces.
type resolveTestFS struct {
	FileSystemBase
}

func (*resolveTestFS) CopyFileRange(pathIn string, fhIn uint64, offIn int64,
	pathOut string, fhOut uint64, offOut int64, size int, flags uint32) int {
	return 0
}
func (*resolveTestFS) Flock(path string, op int, fh uint64) int          { return 0 }
func (*resolveTestFS) Rename3(oldpath, newpath string, flags uint32) int { return 0 }

func TestResolveOps(t *testing.T) {
	o := resolveOps(&resolveTestFS{})
	// implemented -> resolved
	if nil == o.copyFileRange {
		t.Error("copyFileRange should be resolved")
	}
	if nil == o.flock {
		t.Error("flock should be resolved")
	}
	if nil == o.rename3 {
		t.Error("rename3 should be resolved")
	}
	// not implemented -> nil
	if nil != o.openEx {
		t.Error("openEx should be nil")
	}
	if nil != o.getpath {
		t.Error("getpath should be nil")
	}
	if nil != o.chmod3 {
		t.Error("chmod3 should be nil")
	}
	if nil != o.fallocate {
		t.Error("fallocate should be nil")
	}
	if nil != o.lseek {
		t.Error("lseek should be nil")
	}
}

// withMount mounts fsop at a temp directory, waits until it is ready, runs fn
// against the mountpoint, then unmounts. Unix only (FUSE mount).
func withMount(t *testing.T, fsop FileSystemInterface, fn func(mntp string)) {
	t.Helper()
	if "windows" == runtime.GOOS {
		t.Skip("mount-based test; unix only")
	}
	dir, err := os.MkdirTemp("", "cgofuse-test")
	if nil != err {
		t.Fatal(err)
	}
	defer os.Remove(dir)
	mntp := filepath.Join(dir, "m")
	if err := os.Mkdir(mntp, 0755); nil != err {
		t.Fatal(err)
	}
	defer os.Remove(mntp)

	host := NewFileSystemHost(fsop)
	done := make(chan bool)
	go func() {
		host.Mount(mntp, nil)
		done <- true
	}()
	defer func() {
		host.Unmount()
		<-done
	}()

	for i := 0; 100 > i; i++ {
		if _, err := os.Stat(filepath.Join(mntp, "a")); nil == err {
			fn(mntp)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("mount did not become ready")
}

// opsSpyFS exposes a single regular file and records whether the FUSE3-or-FUSE2
// optional ops (Rename3, Chmod3) were dispatched. These dispatch on both build
// variants because hostRename/hostChmod type-assert the optional interface
// regardless of the negotiated FUSE protocol version.
type opsSpyFS struct {
	FileSystemBase
	rename3 bool
	chmod3  bool
}

func (fs *opsSpyFS) Getattr(path string, stat *Stat_t, fh uint64) int {
	// Report the current user as owner: macFUSE enforces permissions (unlike
	// Linux FUSE without default_permissions), so root-owned entries would deny
	// the test user's rename/chmod before they ever reach the filesystem.
	stat.Uid = uint32(os.Getuid())
	stat.Gid = uint32(os.Getgid())
	switch path {
	case "/":
		stat.Mode = S_IFDIR | 0755
	case "/a", "/b":
		stat.Mode = S_IFREG | 0644
	default:
		return -ENOENT
	}
	return 0
}
func (fs *opsSpyFS) Readdir(path string,
	fill func(name string, stat *Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	fill(".", nil, 0)
	fill("..", nil, 0)
	fill("a", nil, 0)
	return 0
}
func (fs *opsSpyFS) Rename3(oldpath, newpath string, flags uint32) int { fs.rename3 = true; return 0 }
func (fs *opsSpyFS) Chmod3(path string, mode uint32, fh uint64) int    { fs.chmod3 = true; return 0 }

func TestOptionalOpsDispatch(t *testing.T) {
	fs := &opsSpyFS{}
	withMount(t, fs, func(mntp string) {
		if err := os.Rename(filepath.Join(mntp, "a"), filepath.Join(mntp, "b")); nil != err {
			t.Errorf("rename: %v", err)
		}
		if err := os.Chmod(filepath.Join(mntp, "b"), 0600); nil != err {
			t.Errorf("chmod: %v", err)
		}
	})
	if !fs.rename3 {
		t.Error("Rename3 was not dispatched via hostOps")
	}
	if !fs.chmod3 {
		t.Error("Chmod3 was not dispatched via hostOps")
	}
}
