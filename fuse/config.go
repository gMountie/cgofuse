/*
 * config.go
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

// NewFileSystemHost creates a file system host.
func NewFileSystemHost(fsop FileSystemInterface) *FileSystemHost {
	host := &FileSystemHost{}
	host.fsop = fsop
	host.ops = resolveOps(fsop)
	// FUSE3 enables auto cache invalidation by default; preserve that unless
	// the file system explicitly opts out via SetCapAutoInvalData.
	host.capAutoInvalData = true
	return host
}

// SetCapCaseInsensitive informs the host that the hosted file system is case insensitive
// [OSX and Windows only].
func (host *FileSystemHost) SetCapCaseInsensitive(value bool) {
	host.capCaseInsensitive = value
}

// SetCapReaddirPlus informs the host that the hosted file system has the readdir-plus
// capability [Linux and Windows only]. A file system that has the readdir-plus capability can send
// full stat information during Readdir, thus avoiding extraneous Getattr calls.
func (host *FileSystemHost) SetCapReaddirPlus(value bool) {
	host.capReaddirPlus = value
}

// SetCapDeleteAccess informs the host that the hosted file system implements Access that
// understands the DELETE_OK flag [Windows only]. A file system can use this capability
// to deny delete access on Windows.
func (host *FileSystemHost) SetCapDeleteAccess(value bool) {
	host.capDeleteAccess = value
}

// SetCapOpenTrunc informs the host that the hosted file system can handle the O_TRUNC
// Open flag [Linux only].
func (host *FileSystemHost) SetCapOpenTrunc(value bool) {
	host.capOpenTrunc = value
}

// SetCapAutoInvalData controls automatic page-cache invalidation [FUSE3 only].
// It is enabled by default: the kernel revalidates a file's attributes (with a
// Getattr) to detect modifications and invalidate stale cached data, which costs
// a Getattr round-trip per read. A file system that drives cache invalidation
// itself (e.g. via Notify) can pass false to suppress those Getattr calls. Must
// be set before Mount is called.
func (host *FileSystemHost) SetCapAutoInvalData(value bool) {
	host.capAutoInvalData = value
}

// SetCapWritebackCache enables the kernel writeback cache [FUSE3 only]. It is
// disabled by default. When enabled the kernel buffers writes and may deliver
// them to Write coalesced, larger, and out of order; the file system must not
// rely on receiving each application write as a distinct call, and should let
// the kernel own the file size and mtime. Must be set before Mount is called.
func (host *FileSystemHost) SetCapWritebackCache(value bool) {
	host.capWritebackCache = value
}

// SetCapExplicitInvalData makes the file system solely responsible for cache
// invalidation [FUSE3 only], disabled by default. Together with
// SetCapAutoInvalData(false) it stops the kernel revalidating on its own; the
// file system must invalidate stale data itself (e.g. via Notify). Must be set
// before Mount is called.
func (host *FileSystemHost) SetCapExplicitInvalData(value bool) {
	host.capExplicitInvalData = value
}

// SetCapCacheSymlinks enables kernel caching of symbolic link targets
// [FUSE3 only], disabled by default. When enabled the kernel caches the target
// returned by Readlink and avoids repeated Readlink calls. Must be set before
// Mount is called.
func (host *FileSystemHost) SetCapCacheSymlinks(value bool) {
	host.capCacheSymlinks = value
}

// SetMaxReadahead sets the maximum readahead in bytes [FUSE3 only]. Zero (the
// default) leaves the libfuse/kernel default in place. Must be set before Mount
// is called.
func (host *FileSystemHost) SetMaxReadahead(value int) {
	host.maxReadahead = value
}

// SetMaxBackground sets the maximum number of outstanding background requests
// the kernel may have in flight [FUSE3 only]. Raising it can improve throughput
// on high-latency transports. Zero (the default) leaves the libfuse default.
// Must be set before Mount is called.
func (host *FileSystemHost) SetMaxBackground(value int) {
	host.maxBackground = value
}

// SetCongestionThreshold sets the kernel congestion threshold: the number of
// pending background requests at which the kernel considers the file system
// congested [FUSE3 only]. Zero (the default) leaves the libfuse default. Must be
// set before Mount is called.
func (host *FileSystemHost) SetCongestionThreshold(value int) {
	host.congestionThreshold = value
}

// SetDirectIO causes the file system to disable page caching [FUSE3 only]. Must be set
// before Mount is called.
func (host *FileSystemHost) SetDirectIO(value bool) {
	host.directIO = value
}

// SetUseIno causes the file system to use its own inode values [FUSE3 only]. Must be set
// before Mount is called.
func (host *FileSystemHost) SetUseIno(value bool) {
	host.useIno = value
}

// SetLibfusePath sets an explicit path to the FUSE library (libfuse / libfuse3
// on UNIX, the WinFsp DLL on Windows) to load, overriding the built-in search.
// This is the programmatic equivalent of the CGOFUSE_LIBFUSE_PATH environment
// variable. Must be set before Mount is called. The FUSE library is loaded once
// per process, so the first mount's value takes effect.
func (host *FileSystemHost) SetLibfusePath(value string) {
	host.libpath = value
}
