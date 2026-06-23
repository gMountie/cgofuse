//go:build !fuse2 && linux
// +build !fuse2,linux

/*
 * host_ops_behavior_test.go
 *
 * Behavioral tests for the fork's FUSE3-only operations (CopyFileRange,
 * Fallocate, Lseek, Flock). The dispatch tests prove these reach the
 * filesystem; these verify they actually *do* the right thing. FUSE3 build,
 * Linux only (the triggers are Linux syscalls and CI has /dev/fuse).
 */

package fuse

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"unsafe"
)

const (
	seekData         = 3   // SEEK_DATA
	seekHole         = 4   // SEEK_HOLE
	sysCopyFileRange = 326 // copy_file_range, linux/amd64
)

// behaviorFS is a one-file ("/a") in-memory filesystem that implements the
// fork's FUSE3 ops with real semantics so behavior can be asserted.
type behaviorFS struct {
	FileSystemBase
	mu        sync.Mutex
	data      []byte
	flockHeld bool
}

func (fs *behaviorFS) Getattr(path string, stat *Stat_t, fh uint64) int {
	stat.Uid = uint32(os.Getuid())
	stat.Gid = uint32(os.Getgid())
	switch path {
	case "/":
		stat.Mode = S_IFDIR | 0755
	case "/a":
		fs.mu.Lock()
		stat.Size = int64(len(fs.data))
		fs.mu.Unlock()
		stat.Mode = S_IFREG | 0644
	default:
		return -ENOENT
	}
	return 0
}

func (fs *behaviorFS) Open(path string, flags int) (int, uint64) { return 0, 1 }

func (fs *behaviorFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if ofst >= int64(len(fs.data)) {
		return 0
	}
	return copy(buff, fs.data[ofst:])
}

func (fs *behaviorFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if end := ofst + int64(len(buff)); end > int64(len(fs.data)) {
		fs.grow(end)
	}
	copy(fs.data[ofst:], buff)
	return len(buff)
}

func (fs *behaviorFS) Truncate(path string, size int64, fh uint64) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.grow(size)
	fs.data = fs.data[:size]
	return 0
}

// grow extends data to at least n bytes (caller holds mu).
func (fs *behaviorFS) grow(n int64) {
	if int64(len(fs.data)) >= n {
		return
	}
	nd := make([]byte, n)
	copy(nd, fs.data)
	fs.data = nd
}

func (fs *behaviorFS) CopyFileRange(pathIn string, fhIn uint64, offIn int64,
	pathOut string, fhOut uint64, offOut int64, size int, flags uint32) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if offIn+int64(size) > int64(len(fs.data)) {
		size = int(int64(len(fs.data)) - offIn)
	}
	if 0 >= size {
		return 0
	}
	fs.grow(offOut + int64(size))
	copy(fs.data[offOut:offOut+int64(size)], fs.data[offIn:offIn+int64(size)])
	return size
}

func (fs *behaviorFS) Fallocate(path string, mode int, off int64, length int64, fh uint64) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.grow(off + length)
	return 0
}

func (fs *behaviorFS) Lseek(path string, off int64, whence int, fh uint64) int64 {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	sz := int64(len(fs.data))
	if off >= sz {
		return int64(-ENXIO)
	}
	switch whence {
	case seekData: // the whole file is data
		return off
	case seekHole: // a single implicit hole at EOF
		return sz
	}
	return int64(-EINVAL)
}

func (fs *behaviorFS) Flock(path string, op int, fh uint64) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if 0 != op&syscall.LOCK_UN {
		fs.flockHeld = false
		return 0
	}
	if fs.flockHeld {
		return -EAGAIN // EWOULDBLOCK
	}
	fs.flockHeld = true
	return 0
}

func TestFuse3OpsBehavior(t *testing.T) {
	fs := &behaviorFS{}
	withMount(t, fs, func(mntp string) {
		fp := filepath.Join(mntp, "a")

		t.Run("Fallocate", func(t *testing.T) {
			f, err := os.OpenFile(fp, os.O_RDWR, 0)
			if nil != err {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()
			if err := syscall.Fallocate(int(f.Fd()), 0, 0, 8192); nil != err {
				t.Fatalf("fallocate: %v", err)
			}
			fi, err := os.Stat(fp)
			if nil != err {
				t.Fatal(err)
			}
			if 8192 > fi.Size() {
				t.Errorf("size after fallocate = %d, want >= 8192", fi.Size())
			}
		})

		t.Run("CopyFileRange", func(t *testing.T) {
			if "amd64" != runtime.GOARCH {
				t.Skipf("copy_file_range syscall number is hardcoded for amd64 (GOARCH=%s)", runtime.GOARCH)
			}
			f, err := os.OpenFile(fp, os.O_RDWR, 0)
			if nil != err {
				t.Fatal(err)
			}
			defer f.Close()
			want := []byte("HELLO-CFR")
			if _, err := f.WriteAt(want, 0); nil != err {
				t.Fatal(err)
			}
			offIn, offOut := int64(0), int64(4096)
			r, _, errno := syscall.Syscall6(sysCopyFileRange,
				f.Fd(), uintptr(unsafe.Pointer(&offIn)),
				f.Fd(), uintptr(unsafe.Pointer(&offOut)),
				uintptr(len(want)), 0)
			if 0 != errno {
				t.Fatalf("copy_file_range: %v", errno)
			}
			if int(r) != len(want) {
				t.Errorf("copied %d bytes, want %d", r, len(want))
			}
			got := make([]byte, len(want))
			if _, err := f.ReadAt(got, 4096); nil != err {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("copied data = %q, want %q", got, want)
			}
		})

		t.Run("Lseek", func(t *testing.T) {
			if err := os.Truncate(fp, 4096); nil != err {
				t.Fatal(err)
			}
			f, err := os.OpenFile(fp, os.O_RDWR, 0)
			if nil != err {
				t.Fatal(err)
			}
			defer f.Close()
			fd := int(f.Fd())
			if off, err := syscall.Seek(fd, 0, seekData); nil != err || 0 != off {
				t.Errorf("SEEK_DATA(0) = %d, %v; want 0, nil", off, err)
			}
			if off, err := syscall.Seek(fd, 0, seekHole); nil != err || 4096 != off {
				t.Errorf("SEEK_HOLE(0) = %d, %v; want 4096, nil", off, err)
			}
		})

		t.Run("FlockContention", func(t *testing.T) {
			f1, err := os.OpenFile(fp, os.O_RDWR, 0)
			if nil != err {
				t.Fatal(err)
			}
			defer f1.Close()
			f2, err := os.OpenFile(fp, os.O_RDWR, 0)
			if nil != err {
				t.Fatal(err)
			}
			defer f2.Close()
			if err := syscall.Flock(int(f1.Fd()), syscall.LOCK_EX); nil != err {
				t.Fatalf("f1 LOCK_EX: %v", err)
			}
			if err := syscall.Flock(int(f2.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != syscall.EWOULDBLOCK {
				t.Errorf("f2 LOCK_EX|LOCK_NB while held = %v, want EWOULDBLOCK", err)
			}
			if err := syscall.Flock(int(f1.Fd()), syscall.LOCK_UN); nil != err {
				t.Fatalf("f1 LOCK_UN: %v", err)
			}
			if err := syscall.Flock(int(f2.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); nil != err {
				t.Errorf("f2 LOCK_EX|LOCK_NB after release = %v, want nil", err)
			}
		})
	})
}
