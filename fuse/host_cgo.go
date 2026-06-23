//go:build cgo
// +build cgo

/*
 * host_cgo.go
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

/*
// macOS defaults to FUSE2 (macFUSE): macFUSE's FUSE3 is a darwin dialect
// (fuse_darwin_attr, struct statfs, changed ops) that does not match the
// upstream libfuse3 wiring. Build with -tags=fuse3 to target FUSE-T instead,
// whose libfuse3 fork is upstream-API-compatible (struct stat / struct statvfs).
#cgo darwin,!fuse3 CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -I/usr/local/include/osxfuse/fuse -I/usr/local/include/fuse
#cgo darwin,fuse3 CFLAGS: -DFUSE_USE_VERSION=39 -D_FILE_OFFSET_BITS=64 -I/usr/local/include/fuse3
// FUSE3 is the default; build with -tags=fuse2 for FUSE2.
#cgo freebsd,!fuse2 CFLAGS: -DFUSE_USE_VERSION=39 -D_FILE_OFFSET_BITS=64 -I/usr/local/include/fuse3
#cgo freebsd,fuse2 CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -I/usr/local/include/fuse
#cgo netbsd CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -D_KERNTYPES
#cgo openbsd CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64
#cgo linux,!fuse2 CFLAGS: -DFUSE_USE_VERSION=39 -D_FILE_OFFSET_BITS=64 -I/usr/include/fuse3
#cgo linux,fuse2 CFLAGS: -DFUSE_USE_VERSION=28 -D_FILE_OFFSET_BITS=64 -I/usr/include/fuse
#cgo linux LDFLAGS: -ldl
#cgo windows CFLAGS: -DFUSE_USE_VERSION=28 -I/usr/local/include/winfsp
	// Use `set CPATH=C:\Program Files (x86)\WinFsp\inc\fuse` on Windows.
	// The flag `I/usr/local/include/winfsp` only works on xgo and docker.

#cgo CFLAGS: -I${SRCDIR}
#include "cgofuse_cgo.h"
*/
import "C"
import "runtime/cgo"
import "unsafe"

type (
	c_bool                    = C.bool
	c_char                    = C.char
	c_fuse_dev_t              = C.fuse_dev_t
	c_fuse_fill_dir_t         = C.fuse_fill_dir_t
	c_fuse_gid_t              = C.fuse_gid_t
	c_fuse_mode_t             = C.fuse_mode_t
	c_fuse_off_t              = C.fuse_off_t
	c_fuse_opt_offset_t       = C.fuse_opt_offset_t
	c_enum_fuse_readdir_flags = C.enum_fuse_readdir_flags
	c_fuse_stat_t             = C.fuse_stat_t
	c_fuse_stat_ex_t          = C.fuse_stat_ex_t
	c_fuse_statvfs_t          = C.fuse_statvfs_t
	c_fuse_timespec_t         = C.fuse_timespec_t
	c_fuse_uid_t              = C.fuse_uid_t
	c_int                     = C.int
	c_int16_t                 = C.int16_t
	c_int32_t                 = C.int32_t
	c_int64_t                 = C.int64_t
	c_int8_t                  = C.int8_t
	c_size_t                  = C.size_t
	c_struct_fuse             = C.struct_fuse
	c_struct_fuse_args        = C.struct_fuse_args
	c_struct_fuse_config      = C.struct_fuse_config
	c_struct_fuse_conn_info   = C.struct_fuse_conn_info
	c_struct_fuse_context     = C.struct_fuse_context
	c_struct_fuse_file_info   = C.struct_fuse_file_info
	c_struct_fuse_opt         = C.struct_fuse_opt
	c_uint16_t                = C.uint16_t
	c_uint32_t                = C.uint32_t
	c_uint64_t                = C.uint64_t
	c_uint8_t                 = C.uint8_t
	c_uintptr_t               = C.uintptr_t
	c_unsigned                = C.unsigned
)

func c_GoString(s *c_char) string {
	return C.GoString(s)
}
func c_CString(s string) *c_char {
	return C.CString(s)
}

func c_malloc(size c_size_t) unsafe.Pointer {
	return C.malloc(size)
}
func c_calloc(count c_size_t, size c_size_t) unsafe.Pointer {
	return C.calloc(count, size)
}
func c_free(p unsafe.Pointer) {
	C.free(p)
}

// hostHandleNew/Del/Get associate a *FileSystemHost with an opaque pointer that
// is stored as the FUSE private_data and recovered in every operation callback.
// A runtime/cgo.Handle resolves the host through a lock-free table, so the
// per-operation hostHandleGet avoids the global mutex the previous map-based
// implementation took on every FUSE call. See host.go for the shared contract.
//
// A cgo.Handle is a small integer (1, 2, 3, ...), not a real address. It must
// never live in an unsafe.Pointer-typed slot: a value below minLegalPointer
// (4096) in a pointer slot fails the runtime's stack pointer adjustment with
// "invalid pointer found on stack" (the invalidptr check is on by default).
// So the handle is boxed in a C allocation and private_data stays a genuine
// pointer; the handle itself only ever travels as a uintptr.
func hostHandleNew(host *FileSystemHost) unsafe.Pointer {
	p := c_malloc(c_size_t(unsafe.Sizeof(uintptr(0))))
	*(*uintptr)(p) = uintptr(cgo.NewHandle(host))
	return p
}
func hostHandleDel(p unsafe.Pointer) {
	cgo.Handle(*(*uintptr)(p)).Delete()
	c_free(p)
}
func hostHandleGet(p unsafe.Pointer) *FileSystemHost {
	return cgo.Handle(*(*uintptr)(p)).Value().(*FileSystemHost)
}

func c_fuse_get_context() *c_struct_fuse_context {
	return C.fuse_get_context()
}
func c_fuse_opt_free_args(args *c_struct_fuse_args) {
	C.fuse_opt_free_args(args)
}

func c_hostAsgnCconninfo(conn *c_struct_fuse_conn_info, host *FileSystemHost) {
	var o C.cgofuse_conn_opts
	o.capCaseInsensitive = C.bool(host.capCaseInsensitive)
	o.capReaddirPlus = C.bool(host.capReaddirPlus)
	o.capDeleteAccess = C.bool(host.capDeleteAccess)
	o.capOpenTrunc = C.bool(host.capOpenTrunc)
	o.capAutoInvalData = C.bool(host.capAutoInvalData)
	o.capWritebackCache = C.bool(host.capWritebackCache)
	o.capExplicitInvalData = C.bool(host.capExplicitInvalData)
	o.capCacheSymlinks = C.bool(host.capCacheSymlinks)
	o.capFlockLocks = C.bool(nil != host.ops.flock)
	o.maxReadahead = C.unsigned(host.maxReadahead)
	o.maxBackground = C.unsigned(host.maxBackground)
	o.congestionThreshold = C.unsigned(host.congestionThreshold)
	C.hostAsgnCconninfo(conn, &o)
}
func c_hostAsgnCconfig(conf *c_struct_fuse_config,
	directIO c_bool,
	useIno c_bool) {
	C.hostAsgnCconfig(conf, directIO, useIno)
}
func c_hostCstatvfsFromFusestatfs(stbuf *c_fuse_statvfs_t,
	bsize c_uint64_t,
	frsize c_uint64_t,
	blocks c_uint64_t,
	bfree c_uint64_t,
	bavail c_uint64_t,
	files c_uint64_t,
	ffree c_uint64_t,
	favail c_uint64_t,
	fsid c_uint64_t,
	flag c_uint64_t,
	namemax c_uint64_t) {
	C.hostCstatvfsFromFusestatfs(stbuf,
		bsize,
		frsize,
		blocks,
		bfree,
		bavail,
		files,
		ffree,
		favail,
		fsid,
		flag,
		namemax)
}
func c_hostCstatFromFusestat(stbuf *c_fuse_stat_t,
	dev c_uint64_t,
	ino c_uint64_t,
	mode c_uint32_t,
	nlink c_uint32_t,
	uid c_uint32_t,
	gid c_uint32_t,
	rdev c_uint64_t,
	size c_int64_t,
	atimSec c_int64_t, atimNsec c_int64_t,
	mtimSec c_int64_t, mtimNsec c_int64_t,
	ctimSec c_int64_t, ctimNsec c_int64_t,
	blksize c_int64_t,
	blocks c_int64_t,
	birthtimSec c_int64_t, birthtimNsec c_int64_t,
	flags c_uint32_t) {
	C.hostCstatFromFusestat(stbuf,
		dev,
		ino,
		mode,
		nlink,
		uid,
		gid,
		rdev,
		size,
		atimSec,
		atimNsec,
		mtimSec,
		mtimNsec,
		ctimSec,
		ctimNsec,
		blksize,
		blocks,
		birthtimSec,
		birthtimNsec,
		flags)
}
func c_hostAsgnCfileinfo(fi *c_struct_fuse_file_info,
	direct_io c_bool,
	keep_cache c_bool,
	nonseekable c_bool,
	fh c_uint64_t) {
	C.hostAsgnCfileinfo(fi,
		direct_io,
		keep_cache,
		nonseekable,
		fh)
}
func c_hostFilldir(filler c_fuse_fill_dir_t,
	buf unsafe.Pointer, name *c_char, stbuf *c_fuse_stat_t, off c_fuse_off_t) c_int {
	return C.hostFilldir(filler, buf, name, stbuf, off)
}
func c_hostStaticInit() {
	C.hostStaticInit()
}
func c_hostFuseInit() c_int {
	return C.hostFuseInit()
}
func c_hostSetLibfusePath(path string) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	C.cgofuse_set_libfuse_path(p)
}
func c_hostMount(argc c_int, argv **c_char, data unsafe.Pointer) c_int {
	return C.hostMount(argc, argv, data)
}
func c_hostUnmount(fuse *c_struct_fuse, mountpoint *c_char) c_int {
	return C.hostUnmount(fuse, mountpoint)
}
func c_hostNotify(fuse *c_struct_fuse, path *c_char, action c_uint32_t) c_int {
	return C.hostNotify(fuse, path, action)
}
func c_hostOptSet(opt *c_struct_fuse_opt,
	templ *c_char, offset c_fuse_opt_offset_t, value c_int) {
	C.hostOptSet(opt, templ, offset, value)
}
func c_hostOptParse(args *c_struct_fuse_args, data unsafe.Pointer, opts *c_struct_fuse_opt,
	nonopts c_bool) c_int {
	return C.hostOptParse(args, data, opts, nonopts)
}

//export go_hostGetattr
func go_hostGetattr(path0 *c_char, stat0 *c_fuse_stat_t) (errc0 c_int) {
	return hostGetattr(path0, stat0, nil)
}

//export go_hostGetattr3
func go_hostGetattr3(path0 *c_char, stat0 *c_fuse_stat_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostGetattr(path0, stat0, fi0)
}

//export go_hostReadlink
func go_hostReadlink(path0 *c_char, buff0 *c_char, size0 c_size_t) (errc0 c_int) {
	return hostReadlink(path0, buff0, size0)
}

//export go_hostMknod
func go_hostMknod(path0 *c_char, mode0 c_fuse_mode_t, dev0 c_fuse_dev_t) (errc0 c_int) {
	return hostMknod(path0, mode0, dev0)
}

//export go_hostMkdir
func go_hostMkdir(path0 *c_char, mode0 c_fuse_mode_t) (errc0 c_int) {
	return hostMkdir(path0, mode0)
}

//export go_hostUnlink
func go_hostUnlink(path0 *c_char) (errc0 c_int) {
	return hostUnlink(path0)
}

//export go_hostRmdir
func go_hostRmdir(path0 *c_char) (errc0 c_int) {
	return hostRmdir(path0)
}

//export go_hostSymlink
func go_hostSymlink(target0 *c_char, newpath0 *c_char) (errc0 c_int) {
	return hostSymlink(target0, newpath0)
}

//export go_hostRename
func go_hostRename(oldpath0 *c_char, newpath0 *c_char) (errc0 c_int) {
	return hostRename(oldpath0, newpath0, 0)
}

//export go_hostRename3
func go_hostRename3(oldpath0 *c_char, newpath0 *c_char, flags c_uint32_t) (errc0 c_int) {
	return hostRename(oldpath0, newpath0, flags)
}

//export go_hostLink
func go_hostLink(oldpath0 *c_char, newpath0 *c_char) (errc0 c_int) {
	return hostLink(oldpath0, newpath0)
}

//export go_hostChmod
func go_hostChmod(path0 *c_char, mode0 c_fuse_mode_t) (errc0 c_int) {
	return hostChmod(path0, mode0, nil)
}

//export go_hostChmod3
func go_hostChmod3(path0 *c_char, mode0 c_fuse_mode_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostChmod(path0, mode0, fi0)
}

//export go_hostChown
func go_hostChown(path0 *c_char, uid0 c_fuse_uid_t, gid0 c_fuse_gid_t) (errc0 c_int) {
	return hostChown(path0, uid0, gid0, nil)
}

//export go_hostChown3
func go_hostChown3(path0 *c_char, uid0 c_fuse_uid_t, gid0 c_fuse_gid_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostChown(path0, uid0, gid0, fi0)
}

//export go_hostTruncate
func go_hostTruncate(path0 *c_char, size0 c_fuse_off_t) (errc0 c_int) {
	return hostTruncate(path0, size0, nil)
}

//export go_hostTruncate3
func go_hostTruncate3(path0 *c_char, size0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostTruncate(path0, size0, fi0)
}

//export go_hostOpen
func go_hostOpen(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostOpen(path0, fi0)
}

//export go_hostRead
func go_hostRead(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 c_int) {
	return hostRead(path0, buff0, size0, ofst0, fi0)
}

//export go_hostWrite
func go_hostWrite(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 c_int) {
	return hostWrite(path0, buff0, size0, ofst0, fi0)
}

//export go_hostStatfs
func go_hostStatfs(path0 *c_char, stat0 *c_fuse_statvfs_t) (errc0 c_int) {
	return hostStatfs(path0, stat0)
}

//export go_hostFlush
func go_hostFlush(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFlush(path0, fi0)
}

//export go_hostRelease
func go_hostRelease(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostRelease(path0, fi0)
}

//export go_hostFsync
func go_hostFsync(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFsync(path0, datasync, fi0)
}

//export go_hostSetxattr
func go_hostSetxattr(path0 *c_char, name0 *c_char, buff0 *c_char, size0 c_size_t,
	flags c_int) (errc0 c_int) {
	return hostSetxattr(path0, name0, buff0, size0, flags)
}

//export go_hostGetxattr
func go_hostGetxattr(path0 *c_char, name0 *c_char, buff0 *c_char, size0 c_size_t) (nbyt0 c_int) {
	return hostGetxattr(path0, name0, buff0, size0)
}

//export go_hostListxattr
func go_hostListxattr(path0 *c_char, buff0 *c_char, size0 c_size_t) (nbyt0 c_int) {
	return hostListxattr(path0, buff0, size0)
}

//export go_hostRemovexattr
func go_hostRemovexattr(path0 *c_char, name0 *c_char) (errc0 c_int) {
	return hostRemovexattr(path0, name0)
}

//export go_hostOpendir
func go_hostOpendir(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostOpendir(path0, fi0)
}

//export go_hostReaddir
func go_hostReaddir(path0 *c_char,
	buff0 unsafe.Pointer, fill0 c_fuse_fill_dir_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostReaddir(path0, buff0, fill0, ofst0, fi0)
}

//export go_hostReaddir3
func go_hostReaddir3(path0 *c_char,
	buff0 unsafe.Pointer, fill0 c_fuse_fill_dir_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info, flags c_enum_fuse_readdir_flags) (errc0 c_int) {
	return hostReaddir(path0, buff0, fill0, ofst0, fi0)
}

//export go_hostReleasedir
func go_hostReleasedir(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostReleasedir(path0, fi0)
}

//export go_hostFsyncdir
func go_hostFsyncdir(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFsyncdir(path0, datasync, fi0)
}

//export go_hostInit
func go_hostInit(conn0 *c_struct_fuse_conn_info) (user_data unsafe.Pointer) {
	return hostInit(conn0, nil)
}

//export go_hostInit3
func go_hostInit3(conn0 *c_struct_fuse_conn_info, conf0 *c_struct_fuse_config) (user_data unsafe.Pointer) {
	return hostInit(conn0, conf0)
}

//export go_hostDestroy
func go_hostDestroy(user_data unsafe.Pointer) {
	hostDestroy(user_data)
}

//export go_hostAccess
func go_hostAccess(path0 *c_char, mask0 c_int) (errc0 c_int) {
	return hostAccess(path0, mask0)
}

//export go_hostCreate
func go_hostCreate(path0 *c_char, mode0 c_fuse_mode_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostCreate(path0, mode0, fi0)
}

//export go_hostFtruncate
func go_hostFtruncate(path0 *c_char, size0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFtruncate(path0, size0, fi0)
}

//export go_hostFgetattr
func go_hostFgetattr(path0 *c_char, stat0 *c_fuse_stat_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFgetattr(path0, stat0, fi0)
}

//export go_hostUtimens
func go_hostUtimens(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	return hostUtimens(path0, tmsp0, nil)
}

//export go_hostUtimens3
func go_hostUtimens3(path0 *c_char, tmsp0 *c_fuse_timespec_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostUtimens(path0, tmsp0, fi0)
}

//export go_hostGetpath
func go_hostGetpath(path0 *c_char, buff0 *c_char, size0 c_size_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostGetpath(path0, buff0, size0, fi0)
}

//export go_hostSetchgtime
func go_hostSetchgtime(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	return hostSetchgtime(path0, tmsp0)
}

//export go_hostSetcrtime
func go_hostSetcrtime(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	return hostSetcrtime(path0, tmsp0)
}

//export go_hostChflags
func go_hostChflags(path0 *c_char, flags c_uint32_t) (errc0 c_int) {
	return hostChflags(path0, flags)
}

//export go_hostCopyFileRange
func go_hostCopyFileRange(pathIn0 *c_char, fiIn0 *c_struct_fuse_file_info, offIn0 c_fuse_off_t,
	pathOut0 *c_char, fiOut0 *c_struct_fuse_file_info, offOut0 c_fuse_off_t,
	size0 c_size_t, flags0 c_int) (nbyt0 c_fuse_off_t) {
	return hostCopyFileRange(pathIn0, fiIn0, offIn0, pathOut0, fiOut0, offOut0, size0, flags0)
}

//export go_hostLseek
func go_hostLseek(path0 *c_char, off0 c_fuse_off_t, whence0 c_int,
	fi0 *c_struct_fuse_file_info) (rslt0 c_fuse_off_t) {
	return hostLseek(path0, off0, whence0, fi0)
}

//export go_hostFallocate
func go_hostFallocate(path0 *c_char, mode0 c_int, off0 c_fuse_off_t, length0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	return hostFallocate(path0, mode0, off0, length0, fi0)
}

//export go_hostFlock
func go_hostFlock(path0 *c_char, fi0 *c_struct_fuse_file_info, op0 c_int) (errc0 c_int) {
	return hostFlock(path0, fi0, op0)
}
