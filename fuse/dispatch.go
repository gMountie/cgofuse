/*
 * dispatch.go
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
	"unsafe"
)

func copyCstatvfsFromFusestatfs(dst *c_fuse_statvfs_t, src *Statfs_t) {
	c_hostCstatvfsFromFusestatfs(dst,
		c_uint64_t(src.Bsize),
		c_uint64_t(src.Frsize),
		c_uint64_t(src.Blocks),
		c_uint64_t(src.Bfree),
		c_uint64_t(src.Bavail),
		c_uint64_t(src.Files),
		c_uint64_t(src.Ffree),
		c_uint64_t(src.Favail),
		c_uint64_t(src.Fsid),
		c_uint64_t(src.Flag),
		c_uint64_t(src.Namemax))
}

func copyCstatFromFusestat(dst *c_fuse_stat_t, src *Stat_t) {
	c_hostCstatFromFusestat(dst,
		c_uint64_t(src.Dev),
		c_uint64_t(src.Ino),
		c_uint32_t(src.Mode),
		c_uint32_t(src.Nlink),
		c_uint32_t(src.Uid),
		c_uint32_t(src.Gid),
		c_uint64_t(src.Rdev),
		c_int64_t(src.Size),
		c_int64_t(src.Atim.Sec), c_int64_t(src.Atim.Nsec),
		c_int64_t(src.Mtim.Sec), c_int64_t(src.Mtim.Nsec),
		c_int64_t(src.Ctim.Sec), c_int64_t(src.Ctim.Nsec),
		c_int64_t(src.Blksize),
		c_int64_t(src.Blocks),
		c_int64_t(src.Birthtim.Sec), c_int64_t(src.Birthtim.Nsec),
		c_uint32_t(src.Flags))
}

func copyFusetimespecFromCtimespec(dst *Timespec, src *c_fuse_timespec_t) {
	dst.Sec = int64(src.tv_sec)
	dst.Nsec = int64(src.tv_nsec)
}

func recoverAsErrno(errc0 *c_int) {
	if r := recover(); nil != r {
		switch e := r.(type) {
		case Error:
			*errc0 = c_int(e)
		default:
			*errc0 = -c_int(EIO)
		}
	}
}

// recoverAsErrno64 is recoverAsErrno for operations whose result is a 64-bit
// signed value (a byte count or file offset) rather than a plain errno.
func recoverAsErrno64(rslt0 *c_fuse_off_t) {
	if r := recover(); nil != r {
		switch e := r.(type) {
		case Error:
			*rslt0 = c_fuse_off_t(e)
		default:
			*rslt0 = -c_fuse_off_t(EIO)
		}
	}
}

func hostGetattr(path0 *c_char, stat0 *c_fuse_stat_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	stat := &Stat_t{}
	fifh := ^uint64(0)
	if nil != fi0 {
		fifh = uint64(fi0.fh)
	}
	errc := fsop.Getattr(path, stat, fifh)
	copyCstatFromFusestat(stat0, stat)
	return c_int(errc)
}

func hostReadlink(path0 *c_char, buff0 *c_char, size0 c_size_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc, rslt := fsop.Readlink(path)
	if 0 < size0 { // size0 is unsigned; size0-1 would underflow when size0==0
		buff := (*[maxwidth]byte)(unsafe.Pointer(buff0))
		copy(buff[:size0-1], rslt)
		rlen := len(rslt)
		if c_size_t(rlen) < size0 {
			buff[rlen] = 0
		}
	}
	return c_int(errc)
}

func hostMknod(path0 *c_char, mode0 c_fuse_mode_t, dev0 c_fuse_dev_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Mknod(path, uint32(mode0), uint64(dev0))
	return c_int(errc)
}

func hostMkdir(path0 *c_char, mode0 c_fuse_mode_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Mkdir(path, uint32(mode0))
	return c_int(errc)
}

func hostUnlink(path0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Unlink(path)
	return c_int(errc)
}

func hostRmdir(path0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Rmdir(path)
	return c_int(errc)
}

func hostSymlink(target0 *c_char, newpath0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	target, newpath := c_GoString(target0), c_GoString(newpath0)
	errc := fsop.Symlink(target, newpath)
	return c_int(errc)
}

func hostRename(oldpath0 *c_char, newpath0 *c_char, flags c_uint32_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	oldpath, newpath := c_GoString(oldpath0), c_GoString(newpath0)
	intf, ok := fsop.(FileSystemRename3)
	if ok {
		errc := intf.Rename3(oldpath, newpath, uint32(flags))
		return c_int(errc)
	} else {
		if 0 != flags {
			// man 2 rename: EINVAL when "the filesystem does not support one of the flags"
			return -c_int(EINVAL)
		}
		errc := fsop.Rename(oldpath, newpath)
		return c_int(errc)
	}
}

func hostLink(oldpath0 *c_char, newpath0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	oldpath, newpath := c_GoString(oldpath0), c_GoString(newpath0)
	errc := fsop.Link(oldpath, newpath)
	return c_int(errc)
}

func hostChmod(path0 *c_char, mode0 c_fuse_mode_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	intf, ok := fsop.(FileSystemChmod3)
	if ok {
		fifh := ^uint64(0)
		if nil != fi0 {
			fifh = uint64(fi0.fh)
		}
		errc := intf.Chmod3(path, uint32(mode0), fifh)
		return c_int(errc)
	} else {
		errc := fsop.Chmod(path, uint32(mode0))
		return c_int(errc)
	}
}

func hostChown(path0 *c_char, uid0 c_fuse_uid_t, gid0 c_fuse_gid_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	intf, ok := fsop.(FileSystemChown3)
	if ok {
		fifh := ^uint64(0)
		if nil != fi0 {
			fifh = uint64(fi0.fh)
		}
		errc := intf.Chown3(path, uint32(uid0), uint32(gid0), fifh)
		return c_int(errc)
	} else {
		errc := fsop.Chown(path, uint32(uid0), uint32(gid0))
		return c_int(errc)
	}
}

func hostTruncate(path0 *c_char, size0 c_fuse_off_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	fifh := ^uint64(0)
	if nil != fi0 {
		fifh = uint64(fi0.fh)
	}
	errc := fsop.Truncate(path, int64(size0), fifh)
	return c_int(errc)
}

func hostOpen(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	intf, ok := fsop.(FileSystemOpenEx)
	if ok {
		fi := FileInfo_t{Flags: int(fi0.flags)}
		errc := intf.OpenEx(path, &fi)
		c_hostAsgnCfileinfo(fi0,
			c_bool(fi.DirectIo),
			c_bool(fi.KeepCache),
			c_bool(fi.NonSeekable),
			c_uint64_t(fi.Fh))
		return c_int(errc)
	} else {
		errc, rslt := fsop.Open(path, int(fi0.flags))
		fi0.fh = c_uint64_t(rslt)
		return c_int(errc)
	}
}

func hostRead(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 c_int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	buff := (*[maxwidth]byte)(unsafe.Pointer(buff0))
	nbyt := fsop.Read(path, buff[:size0], int64(ofst0), uint64(fi0.fh))
	return c_int(nbyt)
}

func hostWrite(path0 *c_char, buff0 *c_char, size0 c_size_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (nbyt0 c_int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	buff := (*[maxwidth]byte)(unsafe.Pointer(buff0))
	nbyt := fsop.Write(path, buff[:size0], int64(ofst0), uint64(fi0.fh))
	return c_int(nbyt)
}

func hostStatfs(path0 *c_char, stat0 *c_fuse_statvfs_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	stat := &Statfs_t{}
	errc := fsop.Statfs(path, stat)
	if -ENOSYS == errc {
		stat = &Statfs_t{}
		errc = 0
	}
	copyCstatvfsFromFusestatfs(stat0, stat)
	return c_int(errc)
}

func hostFlush(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Flush(path, uint64(fi0.fh))
	return c_int(errc)
}

func hostRelease(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Release(path, uint64(fi0.fh))
	return c_int(errc)
}

func hostFsync(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Fsync(path, 0 != datasync, uint64(fi0.fh))
	if -ENOSYS == errc {
		errc = 0
	}
	return c_int(errc)
}

func hostSetxattr(path0 *c_char, name0 *c_char, buff0 *c_char, size0 c_size_t,
	flags c_int) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	name := c_GoString(name0)
	buff := (*[maxwidth]byte)(unsafe.Pointer(buff0))
	errc := fsop.Setxattr(path, name, buff[:size0], int(flags))
	return c_int(errc)
}

func hostGetxattr(path0 *c_char, name0 *c_char, buff0 *c_char, size0 c_size_t) (nbyt0 c_int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	name := c_GoString(name0)
	errc, rslt := fsop.Getxattr(path, name)
	if 0 != errc {
		return c_int(errc)
	}
	if 0 != size0 {
		if len(rslt) > int(size0) {
			return -c_int(ERANGE)
		}
		buff := (*[maxwidth]byte)(unsafe.Pointer(buff0))
		copy(buff[:size0], rslt)
	}
	return c_int(len(rslt))
}

func hostListxattr(path0 *c_char, buff0 *c_char, size0 c_size_t) (nbyt0 c_int) {
	defer recoverAsErrno(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	buff := (*[maxwidth]byte)(unsafe.Pointer(buff0))
	size := int(size0)
	nbyt := 0
	fill := func(name1 string) bool {
		nlen := len(name1)
		if 0 != size {
			if nbyt+nlen+1 > size {
				return false
			}
			copy(buff[nbyt:nbyt+nlen], name1)
			buff[nbyt+nlen] = 0
		}
		nbyt += nlen + 1
		return true
	}
	errc := fsop.Listxattr(path, fill)
	if 0 != errc {
		return c_int(errc)
	}
	return c_int(nbyt)
}

func hostRemovexattr(path0 *c_char, name0 *c_char) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	name := c_GoString(name0)
	errc := fsop.Removexattr(path, name)
	return c_int(errc)
}

func hostOpendir(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc, rslt := fsop.Opendir(path)
	if -ENOSYS == errc {
		errc = 0
	}
	fi0.fh = c_uint64_t(rslt)
	return c_int(errc)
}

func hostReaddir(path0 *c_char, buff0 unsafe.Pointer, fill0 c_fuse_fill_dir_t, ofst0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	fill := func(name1 string, stat1 *Stat_t, off1 int64) bool {
		name := c_CString(name1)
		defer c_free(unsafe.Pointer(name))
		if nil == stat1 {
			return 0 == c_hostFilldir(fill0, buff0, name, nil, c_fuse_off_t(off1))
		} else {
			stat_ex := c_fuse_stat_ex_t{} // support WinFsp fuse_stat_ex
			stat := (*c_fuse_stat_t)(unsafe.Pointer(&stat_ex))
			copyCstatFromFusestat(stat, stat1)
			return 0 == c_hostFilldir(fill0, buff0, name, stat, c_fuse_off_t(off1))
		}
	}
	errc := fsop.Readdir(path, fill, int64(ofst0), uint64(fi0.fh))
	return c_int(errc)
}

func hostReleasedir(path0 *c_char, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Releasedir(path, uint64(fi0.fh))
	return c_int(errc)
}

func hostFsyncdir(path0 *c_char, datasync c_int, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Fsyncdir(path, 0 != datasync, uint64(fi0.fh))
	if -ENOSYS == errc {
		errc = 0
	}
	return c_int(errc)
}

func hostCopyFileRange(pathIn0 *c_char, fiIn0 *c_struct_fuse_file_info, offIn0 c_fuse_off_t,
	pathOut0 *c_char, fiOut0 *c_struct_fuse_file_info, offOut0 c_fuse_off_t,
	size0 c_size_t, flags0 c_int) (nbyt0 c_fuse_off_t) {
	defer recoverAsErrno64(&nbyt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemCopyFileRange)
	if !ok {
		return -c_fuse_off_t(ENOSYS)
	}
	pathIn, pathOut := c_GoString(pathIn0), c_GoString(pathOut0)
	fhIn, fhOut := ^uint64(0), ^uint64(0)
	if nil != fiIn0 {
		fhIn = uint64(fiIn0.fh)
	}
	if nil != fiOut0 {
		fhOut = uint64(fiOut0.fh)
	}
	nbyt := intf.CopyFileRange(pathIn, fhIn, int64(offIn0),
		pathOut, fhOut, int64(offOut0), int(size0), uint32(flags0))
	return c_fuse_off_t(nbyt)
}

func hostLseek(path0 *c_char, off0 c_fuse_off_t, whence0 c_int,
	fi0 *c_struct_fuse_file_info) (rslt0 c_fuse_off_t) {
	defer recoverAsErrno64(&rslt0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemLseek)
	if !ok {
		return -c_fuse_off_t(ENOSYS)
	}
	path := c_GoString(path0)
	fifh := ^uint64(0)
	if nil != fi0 {
		fifh = uint64(fi0.fh)
	}
	rslt := intf.Lseek(path, int64(off0), int(whence0), fifh)
	return c_fuse_off_t(rslt)
}

func hostFallocate(path0 *c_char, mode0 c_int, off0 c_fuse_off_t, length0 c_fuse_off_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemFallocate)
	if !ok {
		return -c_int(ENOSYS)
	}
	path := c_GoString(path0)
	fifh := ^uint64(0)
	if nil != fi0 {
		fifh = uint64(fi0.fh)
	}
	errc := intf.Fallocate(path, int(mode0), int64(off0), int64(length0), fifh)
	return c_int(errc)
}

// hostHasFlock reports whether the file system implements FileSystemFlock, so
// that the FUSE_CAP_FLOCK_LOCKS capability can be negotiated (the kernel only
// forwards flock requests when it is).
func hostHasFlock(fsop FileSystemInterface) bool {
	_, ok := fsop.(FileSystemFlock)
	return ok
}

func hostFlock(path0 *c_char, fi0 *c_struct_fuse_file_info, op0 c_int) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemFlock)
	if !ok {
		return -c_int(ENOSYS)
	}
	path := c_GoString(path0)
	fifh := ^uint64(0)
	if nil != fi0 {
		fifh = uint64(fi0.fh)
	}
	errc := intf.Flock(path, int(op0), fifh)
	return c_int(errc)
}

func hostAccess(path0 *c_char, mask0 c_int) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Access(path, uint32(mask0))
	return c_int(errc)
}

func hostCreate(path0 *c_char, mode0 c_fuse_mode_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	intf, ok := fsop.(FileSystemOpenEx)
	if ok {
		fi := FileInfo_t{Flags: int(fi0.flags)}
		errc := intf.CreateEx(path, uint32(mode0), &fi)
		if -ENOSYS == errc {
			errc = fsop.Mknod(path, S_IFREG|uint32(mode0), 0)
			if 0 == errc {
				errc = intf.OpenEx(path, &fi)
			}
		}
		c_hostAsgnCfileinfo(fi0,
			c_bool(fi.DirectIo),
			c_bool(fi.KeepCache),
			c_bool(fi.NonSeekable),
			c_uint64_t(fi.Fh))
		return c_int(errc)
	} else {
		errc, rslt := fsop.Create(path, int(fi0.flags), uint32(mode0))
		if -ENOSYS == errc {
			errc = fsop.Mknod(path, S_IFREG|uint32(mode0), 0)
			if 0 == errc {
				errc, rslt = fsop.Open(path, int(fi0.flags))
			}
		}
		fi0.fh = c_uint64_t(rslt)
		return c_int(errc)
	}
}

func hostFtruncate(path0 *c_char, size0 c_fuse_off_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	errc := fsop.Truncate(path, int64(size0), uint64(fi0.fh))
	return c_int(errc)
}

func hostFgetattr(path0 *c_char, stat0 *c_fuse_stat_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	stat := &Stat_t{}
	errc := fsop.Getattr(path, stat, uint64(fi0.fh))
	copyCstatFromFusestat(stat0, stat)
	return c_int(errc)
}

func hostUtimens(path0 *c_char, tmsp0 *c_fuse_timespec_t, fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	path := c_GoString(path0)
	tmsp := [2]Timespec{}
	if nil == tmsp0 {
		tmsp[0] = Now()
		tmsp[1] = tmsp[0]
	} else if tmsa := (*[2]c_fuse_timespec_t)(unsafe.Pointer(tmsp0)); UTIME_NOW == tmsa[0].tv_nsec &&
		UTIME_NOW == tmsa[1].tv_nsec {
		tmsp[0] = Now()
		tmsp[1] = tmsp[0]
	} else {
		copyFusetimespecFromCtimespec(&tmsp[0], &tmsa[0])
		copyFusetimespecFromCtimespec(&tmsp[1], &tmsa[1])
	}
	intf, ok := fsop.(FileSystemUtimens3)
	if ok {
		fifh := ^uint64(0)
		if nil != fi0 {
			fifh = uint64(fi0.fh)
		}
		errc := intf.Utimens3(path, tmsp[:], fifh)
		return c_int(errc)
	} else {
		errc := fsop.Utimens(path, tmsp[:])
		return c_int(errc)
	}
}

func hostGetpath(path0 *c_char, buff0 *c_char, size0 c_size_t,
	fi0 *c_struct_fuse_file_info) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemGetpath)
	if !ok {
		return -c_int(ENOSYS)
	}
	path := c_GoString(path0)
	fifh := ^uint64(0)
	if nil != fi0 {
		fifh = uint64(fi0.fh)
	}
	errc, rslt := intf.Getpath(path, fifh)
	if 0 < size0 { // size0 is unsigned; size0-1 would underflow when size0==0
		buff := (*[maxwidth]byte)(unsafe.Pointer(buff0))
		copy(buff[:size0-1], rslt)
		rlen := len(rslt)
		if c_size_t(rlen) < size0 {
			buff[rlen] = 0
		}
	}
	return c_int(errc)
}

func hostSetchgtime(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemSetchgtime)
	if !ok {
		// say we did it!
		return 0
	}
	path := c_GoString(path0)
	tmsp := Timespec{}
	copyFusetimespecFromCtimespec(&tmsp, tmsp0)
	errc := intf.Setchgtime(path, tmsp)
	return c_int(errc)
}

func hostSetcrtime(path0 *c_char, tmsp0 *c_fuse_timespec_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemSetcrtime)
	if !ok {
		// say we did it!
		return 0
	}
	path := c_GoString(path0)
	tmsp := Timespec{}
	copyFusetimespecFromCtimespec(&tmsp, tmsp0)
	errc := intf.Setcrtime(path, tmsp)
	return c_int(errc)
}

func hostChflags(path0 *c_char, flags c_uint32_t) (errc0 c_int) {
	defer recoverAsErrno(&errc0)
	fsop := hostHandleGet(c_fuse_get_context().private_data).fsop
	intf, ok := fsop.(FileSystemChflags)
	if !ok {
		// say we did it!
		return 0
	}
	path := c_GoString(path0)
	errc := intf.Chflags(path, uint32(flags))
	return c_int(errc)
}
