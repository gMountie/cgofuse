//go:build !fuse2 && linux
// +build !fuse2,linux

/*
 * host_ops_fuse3_test.go
 *
 * Dispatch tests for FUSE3-only operations (Fallocate, Flock). These ops are
 * wired into fuse_operations only under FUSE3, so the file is excluded on the
 * -tags=fuse2 build. Linux only (the triggers are Linux syscalls).
 */

package fuse

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

type fuse3SpyFS struct {
	FileSystemBase
	falloc bool
	flock  bool
}

func (fs *fuse3SpyFS) Getattr(path string, stat *Stat_t, fh uint64) int {
	switch path {
	case "/":
		stat.Mode = S_IFDIR | 0755
	case "/a":
		stat.Mode = S_IFREG | 0644
	default:
		return -ENOENT
	}
	return 0
}
func (fs *fuse3SpyFS) Readdir(path string,
	fill func(name string, stat *Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	fill(".", nil, 0)
	fill("..", nil, 0)
	fill("a", nil, 0)
	return 0
}
func (fs *fuse3SpyFS) Open(path string, flags int) (int, uint64) { return 0, 0 }
func (fs *fuse3SpyFS) Fallocate(path string, mode int, off int64, length int64, fh uint64) int {
	fs.falloc = true
	return 0
}
func (fs *fuse3SpyFS) Flock(path string, op int, fh uint64) int { fs.flock = true; return 0 }

func TestFuse3OpsDispatch(t *testing.T) {
	fs := &fuse3SpyFS{}
	withMount(t, fs, func(mntp string) {
		f, err := os.OpenFile(filepath.Join(mntp, "a"), os.O_RDWR, 0)
		if nil != err {
			t.Errorf("open: %v", err)
			return
		}
		defer f.Close()
		fd := int(f.Fd())
		if err := syscall.Fallocate(fd, 0, 0, 4096); nil != err {
			t.Errorf("fallocate: %v", err)
		}
		if err := syscall.Flock(fd, syscall.LOCK_EX); nil != err {
			t.Errorf("flock: %v", err)
		}
	})
	if !fs.falloc {
		t.Error("Fallocate was not dispatched via hostOps")
	}
	if !fs.flock {
		t.Error("Flock was not dispatched via hostOps")
	}
}
