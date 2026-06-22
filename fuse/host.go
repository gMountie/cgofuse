/*
 * host.go
 *
 * Copyright 2017-2022 Bill Zissimopoulos
 */
/*
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

package fuse

import (
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// FileSystemHost is used to host a file system.
type FileSystemHost struct {
	fsop FileSystemInterface
	sigc chan os.Signal

	// hmu guards fuse and mntp, which are written by the FUSE loop thread
	// (hostInit/hostDestroy) while caller threads read them via Unmount/Notify
	// (the signal goroutine installed by Mount calls Unmount).
	hmu  sync.Mutex
	fuse *c_struct_fuse
	mntp string

	capCaseInsensitive   bool
	capReaddirPlus       bool
	capDeleteAccess      bool
	capOpenTrunc         bool
	capAutoInvalData     bool
	capWritebackCache    bool
	capExplicitInvalData bool
	capCacheSymlinks     bool
	maxReadahead         int
	maxBackground        int
	congestionThreshold  int
	directIO             bool
	useIno               bool
}

const maxwidth = 1 << (30 + 10*(^uint(0)>>32&1))

// hostHandleNew, hostHandleDel and hostHandleGet associate a *FileSystemHost
// with an opaque handle that can be passed to the FUSE library as private_data
// and recovered in each operation callback. The implementation is build
// specific: the cgo build uses a lock-free runtime/cgo.Handle, while the nocgo
// (Windows) build uses a guarded map. See host_cgo.go and host_nocgo_windows.go.

func hostInit(conn0 *c_struct_fuse_conn_info, conf0 *c_struct_fuse_config) (user_data unsafe.Pointer) {
	defer func() {
		recover()
	}()
	fctx := c_fuse_get_context()
	user_data = fctx.private_data
	host := hostHandleGet(user_data)
	host.hmu.Lock()
	host.fuse = fctx.fuse
	host.hmu.Unlock()
	c_hostAsgnCconninfo(conn0, host)
	c_hostAsgnCconfig(conf0,
		c_bool(host.directIO),
		c_bool(host.useIno))
	if nil != host.sigc {
		signal.Notify(host.sigc, syscall.SIGINT, syscall.SIGTERM)
	}
	host.fsop.Init()
	return
}

func hostDestroy(user_data unsafe.Pointer) {
	defer func() {
		recover()
	}()
	if "netbsd" == runtime.GOOS {
		user_data = c_fuse_get_context().private_data
	}
	host := hostHandleGet(user_data)
	host.fsop.Destroy()
	if nil != host.sigc {
		signal.Stop(host.sigc)
	}
	host.hmu.Lock()
	host.fuse = nil
	host.hmu.Unlock()
}

// Mount mounts a file system on the given mountpoint with the mount options in opts.
//
// Many of the mount options in opts are specific to the underlying FUSE implementation.
// Some of the common options include:
//
//	-h   --help            print help
//	-V   --version         print FUSE version
//	-d   -o debug          enable FUSE debug output
//	-s                     disable multi-threaded operation
//
// Please refer to the individual FUSE implementation documentation for additional options.
//
// It is allowed for the mountpoint to be the empty string ("") in which case opts is assumed
// to contain the mountpoint. It is also allowed for opts to be nil, although in this case the
// mountpoint must be non-empty.
func (host *FileSystemHost) Mount(mountpoint string, opts []string) bool {
	if 0 == c_hostFuseInit() {
		if "windows" == runtime.GOOS {
			panic("cgofuse: cannot find winfsp")
		} else {
			panic("cgofuse: cannot find FUSE")
		}
	}

	/*
	 * Command line handling
	 *
	 * We must prepare a command line to send to FUSE. This command line will look like this:
	 *
	 *     execname [mountpoint] "-f" [opts...] NULL
	 *
	 * We add the "-f" option because Go cannot handle daemonization (at least on OSX).
	 */
	exec := "<UNKNOWN>"
	if 0 < len(os.Args) {
		exec = os.Args[0]
	}
	argc := len(opts) + 2
	if "" != mountpoint {
		argc++
	}
	argv := make([]*c_char, argc+1)
	argv[0] = c_CString(exec)
	defer c_free(unsafe.Pointer(argv[0]))
	opti := 1
	if "" != mountpoint {
		argv[1] = c_CString(mountpoint)
		defer c_free(unsafe.Pointer(argv[1]))
		opti++
	}
	argv[opti] = c_CString("-f")
	defer c_free(unsafe.Pointer(argv[opti]))
	opti++
	for i := 0; len(opts) > i; i++ {
		argv[i+opti] = c_CString(opts[i])
		defer c_free(unsafe.Pointer(argv[i+opti]))
	}

	/*
	 * Mountpoint extraction
	 *
	 * We need to determine the mountpoint that FUSE is going (to try) to use, so that we
	 * can unmount later.
	 */
	var mntp string
	if "" != mountpoint {
		mntp = mountpoint
	} else {
		outargs, _ := OptParse(opts, "")
		if 1 <= len(outargs) {
			mntp = outargs[0]
		}
	}
	if "" != mntp {
		if "windows" != runtime.GOOS || 2 != len(mntp) || ':' != mntp[1] {
			abs, err := filepath.Abs(mntp)
			if nil == err {
				mntp = abs
			}
		}
	}
	host.hmu.Lock()
	host.mntp = mntp
	host.hmu.Unlock()
	defer func() {
		host.hmu.Lock()
		host.mntp = ""
		host.hmu.Unlock()
	}()

	/*
	 * Handle zombie mounts
	 *
	 * FUSE on UNIX does not automatically unmount the file system, leaving behind "zombie"
	 * mounts. So set things up to always unmount the file system (unless forcibly terminated).
	 * This has the added benefit that the file system Destroy() always gets called.
	 *
	 * On Windows (WinFsp) this is handled by the FUSE layer and we do not have to do anything.
	 */
	if "windows" != runtime.GOOS {
		done := make(chan bool)
		defer func() {
			<-done
		}()
		host.sigc = make(chan os.Signal, 1)
		defer close(host.sigc)
		go func() {
			_, ok := <-host.sigc
			if ok {
				host.Unmount()
			}
			close(done)
		}()
	}

	/*
	 * Tell FUSE to do its job!
	 */
	hndl := hostHandleNew(host)
	defer hostHandleDel(hndl)
	return 0 != c_hostMount(c_int(argc), &argv[0], hndl)
}

// Unmount unmounts a mounted file system.
// Unmount may be called at any time after the Init() method has been called
// and before the Destroy() method has been called.
func (host *FileSystemHost) Unmount() bool {
	host.hmu.Lock()
	fuse := host.fuse
	mntp0 := host.mntp
	host.hmu.Unlock()
	if nil == fuse {
		return false
	}
	var mntp *c_char
	if "" != mntp0 {
		mntp = c_CString(mntp0)
		defer c_free(unsafe.Pointer(mntp))
	}
	return 0 != c_hostUnmount(fuse, mntp)
}

// Notify notifies the operating system about a file change.
// The action is a combination of the fuse.NOTIFY_* constants.
func (host *FileSystemHost) Notify(path string, action uint32) bool {
	host.hmu.Lock()
	fuse := host.fuse
	host.hmu.Unlock()
	if nil == fuse {
		return false
	}
	if "" == path {
		return false
	}
	var p *c_char
	p = c_CString(path)
	defer c_free(unsafe.Pointer(p))
	return 0 != c_hostNotify(fuse, p, c_uint32_t(action))
}

// Getcontext gets information related to a file system operation.
func Getcontext() (uid uint32, gid uint32, pid int) {
	context := c_fuse_get_context()
	uid = uint32(context.uid)
	gid = uint32(context.gid)
	pid = int(context.pid)
	return
}

func init() {
	c_hostStaticInit()
}
