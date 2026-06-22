/*
 * cgofuse_cgo.h
 *
 * Copyright 2017-2022 Bill Zissimopoulos
 */
/*
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

#if !(defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__) || defined(_WIN32))
#error platform not supported
#endif

#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)

#include <dlfcn.h>
#include <pthread.h>
#include <spawn.h>
#include <sys/mount.h>
#include <sys/wait.h>
#include <unistd.h>

#define cgofuse_barrier()         __sync_synchronize()
#define cgofuse_mutex_t           pthread_mutex_t
#define cgofuse_mutex_init(l)     ((void)0)
#define cgofuse_mutex_lock(l)     pthread_mutex_lock(l)
#define cgofuse_mutex_unlock(l)   pthread_mutex_unlock(l)
#define CGOFUSE_MUTEX_INITIALIZER PTHREAD_MUTEX_INITIALIZER

#elif defined(_WIN32)

#include <windows.h>

#define cgofuse_barrier()         MemoryBarrier()
#define cgofuse_mutex_t           CRITICAL_SECTION
#define cgofuse_mutex_init(l)     InitializeCriticalSection(l)
#define cgofuse_mutex_lock(l)     EnterCriticalSection(l)
#define cgofuse_mutex_unlock(l)   LeaveCriticalSection(l)
#define CGOFUSE_MUTEX_INITIALIZER {0}

#endif

static void *cgofuse_init_slow(int hardfail);
static void cgofuse_init_fail(void);
static void *cgofuse_init_fuse(void);

static cgofuse_mutex_t cgofuse_mutex = CGOFUSE_MUTEX_INITIALIZER;
static void *cgofuse_module = 0;

static inline void *cgofuse_init_fast(int hardfail)
{
	void *Module = cgofuse_module;
	cgofuse_barrier();
	if (0 == Module)
		Module = cgofuse_init_slow(hardfail);
	return Module;
}

static void *cgofuse_init_slow(int hardfail)
{
	void *Module;
	cgofuse_mutex_lock(&cgofuse_mutex);
	Module = cgofuse_module;
	if (0 == Module)
	{
		Module = cgofuse_init_fuse();
		cgofuse_barrier();
		cgofuse_module = Module;
	}
	cgofuse_mutex_unlock(&cgofuse_mutex);
	if (0 == Module && hardfail)
		cgofuse_init_fail();
	return Module;
}

static void cgofuse_init_fail(void)
{
#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)
	static const char *message = "cgofuse: cannot find FUSE\n";
	int res = write(2, message, strlen(message));
	(void)res; // suppress dumb gcc warning; see https://gcc.gnu.org/bugzilla/show_bug.cgi?id=66425
	exit(1);
#elif defined(_WIN32)
	static const char *message = "cgofuse: cannot find winfsp\n";
	DWORD BytesTransferred;
	WriteFile(GetStdHandle(STD_ERROR_HANDLE), message, lstrlenA(message), &BytesTransferred, 0);
	ExitProcess(ERROR_DLL_NOT_FOUND);
#endif
}

#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)

#include <fuse.h>

#if defined(__OpenBSD__)
static int (*pfn_fuse_main)(int argc, char *argv[],
    const struct fuse_operations *ops, void *data);
#else
static int (*pfn_fuse_main_real)(int argc, char *argv[],
    const struct fuse_operations *ops, size_t opsize, void *data);
#endif
static struct fuse_context *(*pfn_fuse_get_context)(void);
static int (*pfn_fuse_opt_parse)(struct fuse_args *args, void *data,
    const struct fuse_opt opts[], fuse_opt_proc_t proc);
static void (*pfn_fuse_opt_free_args)(struct fuse_args *args);

static inline int inl_fuse_main_real(int argc, char *argv[],
    const struct fuse_operations *ops, size_t opsize, void *data)
{
	cgofuse_init_fast(1);
#if defined(__OpenBSD__)
	return pfn_fuse_main(argc, argv, ops, data);
#else
	return pfn_fuse_main_real(argc, argv, ops, opsize, data);
#endif
}
static inline struct fuse_context *inl_fuse_get_context(void)
{
	cgofuse_init_fast(1);
	return pfn_fuse_get_context();
}
static inline int inl_fuse_opt_parse(struct fuse_args *args, void *data,
    const struct fuse_opt opts[], fuse_opt_proc_t proc)
{
	cgofuse_init_fast(1);
	return pfn_fuse_opt_parse(args, data, opts, proc);
}
static inline void inl_fuse_opt_free_args(struct fuse_args *args)
{
	cgofuse_init_fast(1);
	return pfn_fuse_opt_free_args(args);
}

#define fuse_main_real     inl_fuse_main_real
#define fuse_exit          fuse_exit_DO_NOT_USE
#define fuse_get_context   inl_fuse_get_context
#define fuse_opt_parse     inl_fuse_opt_parse
#define fuse_opt_free_args inl_fuse_opt_free_args

static void *cgofuse_init_fuse(void)
{
#define CGOFUSE_GET_API(n)                              \
	if (0 == (*(void **)&(pfn_##n) = dlsym(h, #n))) \
		return 0;

	void *h = 0;
#if defined(__APPLE__)
	// runtime path for bundled dylib in e.g. Awesome.app/Contents/Frameworks/libfuse.dylib
	const char *dylib_path = getenv("CGOFUSE_LIBFUSE_PATH");
	if (0 != dylib_path)
		h = dlopen(dylib_path, RTLD_NOW);
	if (0 == h)
		h = dlopen("/usr/local/lib/libfuse.2.dylib", RTLD_NOW); // MacFUSE/OSXFuse >= v4
	if (0 == h)
		h = dlopen("/usr/local/lib/libosxfuse.2.dylib", RTLD_NOW); // MacFUSE/OSXFuse < v4
	if (0 == h)
		h = dlopen("/usr/local/lib/libfuse-t.dylib", RTLD_NOW); // FUSE-T
#elif defined(__FreeBSD__)
#if FUSE_USE_VERSION < 30
	h = dlopen("libfuse.so.2", RTLD_NOW);
#else
	h = dlopen("libfuse3.so.3", RTLD_NOW);
#endif
#elif defined(__NetBSD__)
	h = dlopen("librefuse.so.2", RTLD_NOW);
#elif defined(__OpenBSD__)
	h = dlopen("libfuse.so", RTLD_NOW);
#elif defined(__linux__)
#if FUSE_USE_VERSION < 30
	h = dlopen("libfuse.so.2", RTLD_NOW);
#else
	h = dlopen("libfuse3.so.3", RTLD_NOW);
	if (0 == h)
		h = dlopen("libfuse3.so.4", RTLD_NOW); // some distros ship libfuse3 with this soname
	if (0 == h)
		h = dlopen("libfuse3.so", RTLD_NOW); // dev/unversioned fallback
#endif
#endif
	if (0 == h)
		return 0;

#if defined(__OpenBSD__)
	CGOFUSE_GET_API(fuse_main);
#else
	CGOFUSE_GET_API(fuse_main_real);
#endif
	CGOFUSE_GET_API(fuse_get_context);
	CGOFUSE_GET_API(fuse_opt_parse);
	CGOFUSE_GET_API(fuse_opt_free_args);

	return h;

#undef CGOFUSE_GET_API
}

#elif defined(_WIN32)

#define FSP_FUSE_API           static
#define FSP_FUSE_API_NAME(api) (*pfn_##api)
#define FSP_FUSE_API_CALL(api) (cgofuse_init_fast(1), pfn_##api)
#define FSP_FUSE_SYM(proto, ...) \
	static inline proto      \
	{                        \
		__VA_ARGS__      \
	}
#include <fuse_common.h>
#include <fuse.h>
#include <fuse_opt.h>

// optional
#if !defined(FSP_FUSE_NOTIFY_MKDIR)
static int (*pfn_fsp_fuse_notify)(struct fsp_fuse_env *env,
    struct fuse *f, const char *path, uint32_t action);
#endif

static NTSTATUS FspLoad(void **PModule)
{
#if defined(__aarch64__)
#define FSP_DLLNAME "winfsp-a64.dll"
#elif defined(__amd64__)
#define FSP_DLLNAME "winfsp-x64.dll"
#else
#define FSP_DLLNAME "winfsp-x86.dll"
#endif
#define FSP_DLLPATH "bin\\" FSP_DLLNAME

	WCHAR PathBuf[MAX_PATH];
	DWORD Size;
	DWORD RegType;
	HKEY RegKey;
	LONG Result;
	HMODULE Module;

	if (0 != PModule)
		*PModule = 0;

	Module = LoadLibraryW(L"" FSP_DLLNAME);
	if (0 == Module)
	{
		Result = RegOpenKeyExW(HKEY_LOCAL_MACHINE, L"Software\\WinFsp",
		    0, KEY_READ | KEY_WOW64_32KEY, &RegKey);
		if (ERROR_SUCCESS == Result)
		{
			Size = sizeof PathBuf - sizeof L"" FSP_DLLPATH + sizeof(WCHAR);
			Result = RegQueryValueExW(RegKey, L"InstallDir", 0,
			    &RegType, (LPBYTE)PathBuf, &Size);
			RegCloseKey(RegKey);
			if (ERROR_SUCCESS == Result && REG_SZ != RegType)
				Result = ERROR_FILE_NOT_FOUND;
		}
		if (ERROR_SUCCESS != Result)
			return 0xC0000034; // STATUS_OBJECT_NAME_NOT_FOUND

		if (0 < Size && L'\0' == PathBuf[Size / sizeof(WCHAR) - 1])
			Size -= sizeof(WCHAR);

		RtlCopyMemory(PathBuf + Size / sizeof(WCHAR),
		    L"" FSP_DLLPATH, sizeof L"" FSP_DLLPATH);
		Module = LoadLibraryW(PathBuf);
		if (0 == Module)
			return 0xC0000135; // STATUS_DLL_NOT_FOUND
	}

	if (0 != PModule)
		*PModule = Module;

	return 0; // STATUS_SUCCESS

#undef FSP_DLLNAME
#undef FSP_DLLPATH
}

static void *cgofuse_init_fuse(void)
{
#define CGOFUSE_GET_API(n)                                                       \
	if (0 == (*(void **)&(pfn_fsp_##n) = GetProcAddress(Module, "fsp_" #n))) \
		return 0;

	void *Module;
	NTSTATUS Result = FspLoad(&Module);
	if (0 > Result)
		return 0;

	CGOFUSE_GET_API(fuse_main_real);
	CGOFUSE_GET_API(fuse_exit);
	CGOFUSE_GET_API(fuse_get_context);
	CGOFUSE_GET_API(fuse_opt_parse);
	CGOFUSE_GET_API(fuse_opt_free_args);

	// optional
	*(void **)&pfn_fsp_fuse_notify = GetProcAddress(Module, "fsp_fuse_notify");

	return Module;

#undef CGOFUSE_GET_API
}

static BOOLEAN cgofuse_stat_ex = FALSE;

#endif

#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__) || defined(__linux__)
typedef struct stat fuse_stat_t;
typedef struct stat fuse_stat_ex_t;
typedef struct statvfs fuse_statvfs_t;
typedef struct timespec fuse_timespec_t;
typedef mode_t fuse_mode_t;
typedef dev_t fuse_dev_t;
typedef uid_t fuse_uid_t;
typedef gid_t fuse_gid_t;
typedef off_t fuse_off_t;
typedef unsigned long fuse_opt_offset_t;
#elif defined(_WIN32)
typedef struct fuse_stat fuse_stat_t;
typedef struct fuse_stat_ex fuse_stat_ex_t;
typedef struct fuse_statvfs fuse_statvfs_t;
typedef struct fuse_timespec fuse_timespec_t;
typedef unsigned int fuse_opt_offset_t;
#endif

#if FUSE_USE_VERSION < 30
struct fuse_config;
enum fuse_readdir_flags
{
	fuse_readdir_flags_DUMMY
};
#endif

#if FUSE_USE_VERSION < 30
extern int go_hostGetattr(char *path, fuse_stat_t *stbuf);
#else
extern int go_hostGetattr3(char *path, fuse_stat_t *stbuf, struct fuse_file_info *fi);
#endif
extern int go_hostReadlink(char *path, char *buf, size_t size);
extern int go_hostMknod(char *path, fuse_mode_t mode, fuse_dev_t dev);
extern int go_hostMkdir(char *path, fuse_mode_t mode);
extern int go_hostUnlink(char *path);
extern int go_hostRmdir(char *path);
extern int go_hostSymlink(char *target, char *newpath);
#if FUSE_USE_VERSION < 30
extern int go_hostRename(char *oldpath, char *newpath);
#else
extern int go_hostRename3(char *oldpath, char *newpath, unsigned int flags);
#endif
extern int go_hostLink(char *oldpath, char *newpath);
#if FUSE_USE_VERSION < 30
extern int go_hostChmod(char *path, fuse_mode_t mode);
extern int go_hostChown(char *path, fuse_uid_t uid, fuse_gid_t gid);
extern int go_hostTruncate(char *path, fuse_off_t size);
#else
extern int go_hostChmod3(char *path, fuse_mode_t mode, struct fuse_file_info *fi);
extern int go_hostChown3(char *path, fuse_uid_t uid, fuse_gid_t gid, struct fuse_file_info *fi);
extern int go_hostTruncate3(char *path, fuse_off_t size, struct fuse_file_info *fi);
#endif
extern int go_hostOpen(char *path, struct fuse_file_info *fi);
extern int go_hostRead(char *path, char *buf, size_t size, fuse_off_t off,
    struct fuse_file_info *fi);
extern int go_hostWrite(char *path, char *buf, size_t size, fuse_off_t off,
    struct fuse_file_info *fi);
extern int go_hostStatfs(char *path, fuse_statvfs_t *stbuf);
extern int go_hostFlush(char *path, struct fuse_file_info *fi);
extern int go_hostRelease(char *path, struct fuse_file_info *fi);
extern int go_hostFsync(char *path, int datasync, struct fuse_file_info *fi);
extern int go_hostSetxattr(char *path, char *name, char *value, size_t size, int flags);
extern int go_hostGetxattr(char *path, char *name, char *value, size_t size);
extern int go_hostListxattr(char *path, char *namebuf, size_t size);
extern int go_hostRemovexattr(char *path, char *name);
extern int go_hostOpendir(char *path, struct fuse_file_info *fi);
#if FUSE_USE_VERSION < 30
extern int go_hostReaddir(char *path, void *buf, fuse_fill_dir_t filler, fuse_off_t off,
    struct fuse_file_info *fi);
#else
extern int go_hostReaddir3(char *path, void *buf, fuse_fill_dir_t filler, fuse_off_t off,
    struct fuse_file_info *fi, enum fuse_readdir_flags flags);
#endif
extern int go_hostReleasedir(char *path, struct fuse_file_info *fi);
extern int go_hostFsyncdir(char *path, int datasync, struct fuse_file_info *fi);
#if FUSE_USE_VERSION < 30
extern void *go_hostInit(struct fuse_conn_info *conn);
#else
extern void *go_hostInit3(struct fuse_conn_info *conn, struct fuse_config *conf);
#endif
extern void go_hostDestroy(void *data);
extern int go_hostAccess(char *path, int mask);
extern int go_hostCreate(char *path, fuse_mode_t mode, struct fuse_file_info *fi);
#if FUSE_USE_VERSION < 30
extern int go_hostFtruncate(char *path, fuse_off_t off, struct fuse_file_info *fi);
extern int go_hostFgetattr(char *path, fuse_stat_t *stbuf, struct fuse_file_info *fi);
#endif
// extern int go_hostLock(char *path, struct fuse_file_info *fi, int cmd, struct fuse_flock *lock);
#if FUSE_USE_VERSION < 30
extern int go_hostUtimens(char *path, fuse_timespec_t tv[2]);
#else
extern int go_hostUtimens3(char *path, fuse_timespec_t tv[2], struct fuse_file_info *fi);
#endif
extern int go_hostGetpath(char *path, char *buf, size_t size,
    struct fuse_file_info *fi);
extern int go_hostSetchgtime(char *path, fuse_timespec_t *tv);
extern int go_hostSetcrtime(char *path, fuse_timespec_t *tv);
extern int go_hostChflags(char *path, uint32_t flags);
extern fuse_off_t go_hostCopyFileRange(char *path_in, struct fuse_file_info *fi_in, fuse_off_t off_in,
    char *path_out, struct fuse_file_info *fi_out, fuse_off_t off_out, size_t size, int flags);
extern fuse_off_t go_hostLseek(char *path, fuse_off_t off, int whence, struct fuse_file_info *fi);
extern int go_hostFallocate(char *path, int mode, fuse_off_t off, fuse_off_t length,
    struct fuse_file_info *fi);
extern int go_hostFlock(char *path, struct fuse_file_info *fi, int op);

// cgofuse_conn_opts carries the connection capabilities/tuning from Go to C as
// one value, instead of a long positional argument list.
typedef struct cgofuse_conn_opts
{
	bool capCaseInsensitive, capReaddirPlus, capDeleteAccess, capOpenTrunc;
	bool capAutoInvalData, capWritebackCache, capExplicitInvalData, capCacheSymlinks, capFlockLocks;
	unsigned maxReadahead, maxBackground, congestionThreshold;
} cgofuse_conn_opts;

static inline void hostAsgnCconninfo(struct fuse_conn_info *conn, cgofuse_conn_opts *o)
{
#if defined(__APPLE__)
	if (o->capCaseInsensitive)
		FUSE_ENABLE_CASE_INSENSITIVE(conn);
#elif defined(__NetBSD__) || defined(__OpenBSD__)
#elif defined(__FreeBSD__) || defined(__linux__)
#if FUSE_USE_VERSION >= 30
	if (o->capReaddirPlus)
		conn->want |= conn->capable & FUSE_CAP_READDIRPLUS;
	else
		conn->want &= ~FUSE_CAP_READDIRPLUS;
#endif
	// FUSE_CAP_ATOMIC_O_TRUNC was disabled in FUSE2 and is enabled in FUSE3.
	// So disable it here, unless the user explicitly enables it.
	if (o->capOpenTrunc)
		conn->want |= conn->capable & FUSE_CAP_ATOMIC_O_TRUNC;
	else
		conn->want &= ~FUSE_CAP_ATOMIC_O_TRUNC;
#elif defined(_WIN32)
#if defined(FSP_FUSE_CAP_STAT_EX)
	conn->want |= conn->capable & FSP_FUSE_CAP_STAT_EX;
	cgofuse_stat_ex = 0 != (conn->want & FSP_FUSE_CAP_STAT_EX); // hack!
#endif
	if (o->capCaseInsensitive)
		conn->want |= conn->capable & FSP_FUSE_CAP_CASE_INSENSITIVE;
	if (o->capReaddirPlus)
		conn->want |= conn->capable & FSP_FUSE_CAP_READDIR_PLUS;
	if (o->capDeleteAccess)
		conn->want |= conn->capable & (1 << 24); // FSP_FUSE_CAP_DELETE_ACCESS
#endif

	// Capabilities and tuning common to libfuse platforms (macOS, FreeBSD, Linux).
	// Each is guarded so it compiles to a no-op where the feature is absent.
#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__linux__)
#if defined(FUSE_CAP_AUTO_INVAL_DATA)
	// Enabled by default in FUSE3: the kernel Getattr-revalidates before reads.
	if (o->capAutoInvalData)
		conn->want |= conn->capable & FUSE_CAP_AUTO_INVAL_DATA;
	else
		conn->want &= ~FUSE_CAP_AUTO_INVAL_DATA;
#endif
#if defined(FUSE_CAP_WRITEBACK_CACHE)
	// Opt-in: the kernel buffers and coalesces writes (changes write semantics).
	if (o->capWritebackCache)
		conn->want |= conn->capable & FUSE_CAP_WRITEBACK_CACHE;
	else
		conn->want &= ~FUSE_CAP_WRITEBACK_CACHE;
#endif
#if defined(FUSE_CAP_EXPLICIT_INVAL_DATA)
	// Opt-in: only the file system invalidates cached data (pairs with
	// disabling FUSE_CAP_AUTO_INVAL_DATA).
	if (o->capExplicitInvalData)
		conn->want |= conn->capable & FUSE_CAP_EXPLICIT_INVAL_DATA;
	else
		conn->want &= ~FUSE_CAP_EXPLICIT_INVAL_DATA;
#endif
#if defined(FUSE_CAP_CACHE_SYMLINKS)
	// Opt-in: the kernel caches symlink targets, avoiding repeated Readlink.
	if (o->capCacheSymlinks)
		conn->want |= conn->capable & FUSE_CAP_CACHE_SYMLINKS;
	else
		conn->want &= ~FUSE_CAP_CACHE_SYMLINKS;
#endif
#if defined(FUSE_CAP_FLOCK_LOCKS)
	// Enabled when the file system implements FileSystemFlock; the kernel only
	// forwards flock requests when this capability is negotiated.
	if (o->capFlockLocks)
		conn->want |= conn->capable & FUSE_CAP_FLOCK_LOCKS;
	else
		conn->want &= ~FUSE_CAP_FLOCK_LOCKS;
#endif
	// Tuning knobs; 0 leaves the libfuse/kernel default in place.
	if (0 != o->maxReadahead)
		conn->max_readahead = o->maxReadahead;
	if (0 != o->maxBackground)
		conn->max_background = o->maxBackground;
	if (0 != o->congestionThreshold)
		conn->congestion_threshold = o->congestionThreshold;
#endif
}

#if FUSE_USE_VERSION < 30
static inline void hostAsgnCconfig(struct fuse_config *conf,
    bool direct_io,
    bool use_ino)
{
}
#else
static inline void hostAsgnCconfig(struct fuse_config *conf,
    bool direct_io,
    bool use_ino)
{
	memset(conf, 0, sizeof *conf);
	conf->direct_io = direct_io;
	conf->use_ino = use_ino;
}
#endif

static inline void hostCstatvfsFromFusestatfs(fuse_statvfs_t *stbuf,
    uint64_t bsize,
    uint64_t frsize,
    uint64_t blocks,
    uint64_t bfree,
    uint64_t bavail,
    uint64_t files,
    uint64_t ffree,
    uint64_t favail,
    uint64_t fsid,
    uint64_t flag,
    uint64_t namemax)
{
	memset(stbuf, 0, sizeof *stbuf);
	stbuf->f_bsize = bsize;
	stbuf->f_frsize = frsize;
	stbuf->f_blocks = blocks;
	stbuf->f_bfree = bfree;
	stbuf->f_bavail = bavail;
	stbuf->f_files = files;
	stbuf->f_ffree = ffree;
	stbuf->f_favail = favail;
	stbuf->f_fsid = fsid;
	stbuf->f_flag = flag;
	stbuf->f_namemax = namemax;
}

static inline void hostCstatFromFusestat(fuse_stat_t *stbuf,
    uint64_t dev,
    uint64_t ino,
    uint32_t mode,
    uint32_t nlink,
    uint32_t uid,
    uint32_t gid,
    uint64_t rdev,
    int64_t size,
    int64_t atimSec, int64_t atimNsec,
    int64_t mtimSec, int64_t mtimNsec,
    int64_t ctimSec, int64_t ctimNsec,
    int64_t blksize,
    int64_t blocks,
    int64_t birthtimSec, int64_t birthtimNsec,
    uint32_t flags)
{
	memset(stbuf, 0, sizeof *stbuf);
	stbuf->st_dev = dev;
	stbuf->st_ino = ino;
	stbuf->st_mode = mode;
	stbuf->st_nlink = nlink;
	stbuf->st_uid = uid;
	stbuf->st_gid = gid;
	stbuf->st_rdev = rdev;
	stbuf->st_size = size;
	stbuf->st_blksize = blksize;
	stbuf->st_blocks = blocks;
#if defined(__APPLE__)
	stbuf->st_atimespec.tv_sec = atimSec;
	stbuf->st_atimespec.tv_nsec = atimNsec;
	stbuf->st_mtimespec.tv_sec = mtimSec;
	stbuf->st_mtimespec.tv_nsec = mtimNsec;
	stbuf->st_ctimespec.tv_sec = ctimSec;
	stbuf->st_ctimespec.tv_nsec = ctimNsec;
	if (0 != birthtimSec)
	{
		stbuf->st_birthtimespec.tv_sec = birthtimSec;
		stbuf->st_birthtimespec.tv_nsec = birthtimNsec;
	}
	else
	{
		stbuf->st_birthtimespec.tv_sec = ctimSec;
		stbuf->st_birthtimespec.tv_nsec = ctimNsec;
	}
	stbuf->st_flags = flags;
#elif defined(_WIN32)
	stbuf->st_atim.tv_sec = atimSec;
	stbuf->st_atim.tv_nsec = atimNsec;
	stbuf->st_mtim.tv_sec = mtimSec;
	stbuf->st_mtim.tv_nsec = mtimNsec;
	stbuf->st_ctim.tv_sec = ctimSec;
	stbuf->st_ctim.tv_nsec = ctimNsec;
	if (0 != birthtimSec)
	{
		stbuf->st_birthtim.tv_sec = birthtimSec;
		stbuf->st_birthtim.tv_nsec = birthtimNsec;
	}
	else
	{
		stbuf->st_birthtim.tv_sec = ctimSec;
		stbuf->st_birthtim.tv_nsec = ctimNsec;
	}
#if defined(FSP_FUSE_CAP_STAT_EX)
	if (cgofuse_stat_ex)
		((struct fuse_stat_ex *)stbuf)->st_flags = flags;
#endif
#else
	stbuf->st_atim.tv_sec = atimSec;
	stbuf->st_atim.tv_nsec = atimNsec;
	stbuf->st_mtim.tv_sec = mtimSec;
	stbuf->st_mtim.tv_nsec = mtimNsec;
	stbuf->st_ctim.tv_sec = ctimSec;
	stbuf->st_ctim.tv_nsec = ctimNsec;
#endif
}

static inline void hostAsgnCfileinfo(struct fuse_file_info *fi,
    bool direct_io,
    bool keep_cache,
    bool nonseekable,
    uint64_t fh)
{
	fi->direct_io = direct_io;
	fi->keep_cache = keep_cache;
#if !defined(__NetBSD__)
	fi->nonseekable = nonseekable;
#endif
	fi->fh = fh;
}

static inline int hostFilldir(fuse_fill_dir_t filler, void *buf,
    char *name, fuse_stat_t *stbuf, fuse_off_t off)
{
#if FUSE_USE_VERSION < 30
	return filler(buf, name, stbuf, off);
#else
	return filler(buf, name, stbuf, off, FUSE_FILL_DIR_PLUS);
#endif
}

#if defined(__APPLE__)
static int _hostSetxattr(char *path, char *name, char *value, size_t size, int flags,
    uint32_t position)
{
	// OSX uses position only for the resource fork; we do not support it!
	return go_hostSetxattr(path, name, value, size, flags);
}
static int _hostGetxattr(char *path, char *name, char *value, size_t size,
    uint32_t position)
{
	// OSX uses position only for the resource fork; we do not support it!
	return go_hostGetxattr(path, name, value, size);
}
#else
#define _hostSetxattr go_hostSetxattr
#define _hostGetxattr go_hostGetxattr
#endif

// hostStaticInit, hostFuseInit and hostInit serve different purposes.
//
// hostStaticInit and hostFuseInit are needed to provide static and dynamic initialization
// of the FUSE layer. This is currently useful on Windows only.
//
// hostInit is simply the .init implementation of struct fuse_operations.

static void hostStaticInit(void)
{
	cgofuse_mutex_init(&cgofuse_mutex);
}

static int hostFuseInit(void)
{
	return 0 != cgofuse_init_fast(0);
}

static int hostMount(int argc, char *argv[], void *data)
{
	static struct fuse_operations fsop =
	    {
#if FUSE_USE_VERSION < 30
	        .getattr = (int (*)(const char *, fuse_stat_t *))go_hostGetattr,
#else
	        .getattr = (int (*)(const char *, fuse_stat_t *, struct fuse_file_info *))go_hostGetattr3,
#endif
	        .readlink = (int (*)(const char *, char *, size_t))go_hostReadlink,
	        .mknod = (int (*)(const char *, fuse_mode_t, fuse_dev_t))go_hostMknod,
	        .mkdir = (int (*)(const char *, fuse_mode_t))go_hostMkdir,
	        .unlink = (int (*)(const char *))go_hostUnlink,
	        .rmdir = (int (*)(const char *))go_hostRmdir,
	        .symlink = (int (*)(const char *, const char *))go_hostSymlink,
#if FUSE_USE_VERSION < 30
	        .rename = (int (*)(const char *, const char *))go_hostRename,
#else
	        .rename = (int (*)(const char *, const char *, unsigned int flags))go_hostRename3,
#endif
	        .link = (int (*)(const char *, const char *))go_hostLink,
#if FUSE_USE_VERSION < 30
	        .chmod = (int (*)(const char *, fuse_mode_t))go_hostChmod,
	        .chown = (int (*)(const char *, fuse_uid_t, fuse_gid_t))go_hostChown,
	        .truncate = (int (*)(const char *, fuse_off_t))go_hostTruncate,
#else
	        .chmod = (int (*)(const char *, fuse_mode_t, struct fuse_file_info *))go_hostChmod3,
	        .chown = (int (*)(const char *, fuse_uid_t, fuse_gid_t, struct fuse_file_info *))go_hostChown3,
	        .truncate = (int (*)(const char *, fuse_off_t, struct fuse_file_info *))go_hostTruncate3,
#endif
	        .open = (int (*)(const char *, struct fuse_file_info *))go_hostOpen,
	        .read = (int (*)(const char *, char *, size_t, fuse_off_t, struct fuse_file_info *))
	            go_hostRead,
	        .write = (int (*)(const char *, const char *, size_t, fuse_off_t, struct fuse_file_info *))
	            go_hostWrite,
	        .statfs = (int (*)(const char *, fuse_statvfs_t *))go_hostStatfs,
	        .flush = (int (*)(const char *, struct fuse_file_info *))go_hostFlush,
	        .release = (int (*)(const char *, struct fuse_file_info *))go_hostRelease,
	        .fsync = (int (*)(const char *, int, struct fuse_file_info *))go_hostFsync,
#if defined(__APPLE__)
	        .setxattr = (int (*)(const char *, const char *, const char *, size_t, int, uint32_t))
	            _hostSetxattr,
	        .getxattr = (int (*)(const char *, const char *, char *, size_t, uint32_t))
	            _hostGetxattr,
#else
	        .setxattr = (int (*)(const char *, const char *, const char *, size_t, int))_hostSetxattr,
	        .getxattr = (int (*)(const char *, const char *, char *, size_t))_hostGetxattr,
#endif
	        .listxattr = (int (*)(const char *, char *, size_t))go_hostListxattr,
	        .removexattr = (int (*)(const char *, const char *))go_hostRemovexattr,
	        .opendir = (int (*)(const char *, struct fuse_file_info *))go_hostOpendir,
#if FUSE_USE_VERSION < 30
	        .readdir = (int (*)(const char *, void *, fuse_fill_dir_t, fuse_off_t,
	            struct fuse_file_info *))go_hostReaddir,
#else
	        .readdir = (int (*)(const char *, void *, fuse_fill_dir_t, fuse_off_t,
	            struct fuse_file_info *, enum fuse_readdir_flags flags))go_hostReaddir3,
#endif
	        .releasedir = (int (*)(const char *, struct fuse_file_info *))go_hostReleasedir,
	        .fsyncdir = (int (*)(const char *, int, struct fuse_file_info *))go_hostFsyncdir,
#if FUSE_USE_VERSION < 30
	        .init = (void *(*)(struct fuse_conn_info *))go_hostInit,
#else
	        .init = (void *(*)(struct fuse_conn_info *, struct fuse_config *))go_hostInit3,
#endif
	        .destroy = (void (*)(void *))go_hostDestroy,
	        .access = (int (*)(const char *, int))go_hostAccess,
	        .create = (int (*)(const char *, fuse_mode_t, struct fuse_file_info *))go_hostCreate,
#if FUSE_USE_VERSION < 30
	        .ftruncate = (int (*)(const char *, fuse_off_t, struct fuse_file_info *))go_hostFtruncate,
	        .fgetattr = (int (*)(const char *, fuse_stat_t *, struct fuse_file_info *))go_hostFgetattr,
#endif
	//.lock = (int (*)(const char *, struct fuse_file_info *, int, struct fuse_flock *))
	//	go_hostFlock,
#if FUSE_USE_VERSION < 30
	        .utimens = (int (*)(const char *, const fuse_timespec_t[2]))go_hostUtimens,
#else
	        .utimens = (int (*)(const char *, const fuse_timespec_t[2], struct fuse_file_info *))go_hostUtimens3,
#endif
#if defined(__APPLE__) || (defined(_WIN32) && defined(FSP_FUSE_CAP_STAT_EX))
	        .setchgtime = (int (*)(const char *, const fuse_timespec_t *))go_hostSetchgtime,
	        .setcrtime = (int (*)(const char *, const fuse_timespec_t *))go_hostSetcrtime,
	        .chflags = (int (*)(const char *, uint32_t))go_hostChflags,
#endif
#if FUSE_USE_VERSION >= 30 && defined(FUSE_VERSION) && FUSE_VERSION >= 304
	        .copy_file_range = (ssize_t (*)(const char *, struct fuse_file_info *, off_t,
	            const char *, struct fuse_file_info *, off_t, size_t, int))go_hostCopyFileRange,
#endif
#if FUSE_USE_VERSION >= 30 && defined(FUSE_VERSION) && FUSE_VERSION >= 308
	        .lseek = (off_t (*)(const char *, off_t, int, struct fuse_file_info *))go_hostLseek,
#endif
#if FUSE_USE_VERSION >= 30
	        .fallocate = (int (*)(const char *, int, off_t, off_t, struct fuse_file_info *))go_hostFallocate,
	        .flock = (int (*)(const char *, struct fuse_file_info *, int))go_hostFlock,
#endif
	    };
#if defined(_WIN32)
	// WinFsp introduced the getpath operation in version 2022+ARM64 Beta2,
	// which we would like to use if available.
	//
	// Versions of WinFsp with getpath support have getpath in struct fuse_operations.
	// Versions of WinFsp without getpath support have reserved00 in struct fuse_operations.
	// Unfortunately there is currently no way to detect whether the version of WinFsp we
	// are building against has getpath or not. We would also like to always build with
	// getpath support regardless of the version of WinFsp we are building against.
	//
	// (Ideally a macro should be added to WinFsp <fuse.h> that indicates whether getpath
	// exists.)
	//
	// To resolve this problem we overwrite the location of the getpath/reserved00 field
	// using the hack below. We must make sure to write to the correct location for both
	// 64-bit and 32-bit mode.
	//
	// Note that this is threadsafe in the presence of multiple threads, because we always
	// write the same value to getpath/reserved00 (and because writes of aligned pointer
	// values are atomic so that no half writes can be observed).
	((void **)&fsop)[45] = go_hostGetpath;
#endif
	return 0 == fuse_main_real(argc, argv, &fsop, sizeof fsop, data);
}

static int hostUnmount(struct fuse *fuse, char *mountpoint)
{
#if defined(__APPLE__) || defined(__FreeBSD__) || defined(__NetBSD__) || defined(__OpenBSD__)
	if (0 == mountpoint)
		return 0;
	// darwin,freebsd,netbsd: unmount is available to non-root
	// openbsd: kern.usermount has been removed and mount/unmount is available to root only
	return 0 == unmount(mountpoint, MNT_FORCE);
#elif defined(__linux__)
	if (0 == mountpoint)
		return 0;
	// linux: try umount2 first in case we are root
	if (0 == umount2(mountpoint, MNT_DETACH))
		return 1;
	// linux: umount2 failed; try fusermount
	char *paths[] =
	    {
	        "/bin/fusermount",
	        "/usr/bin/fusermount",
	    };
	char *path = paths[0];
	for (size_t i = 0; sizeof paths / sizeof paths[0] > i; i++)
		if (0 == access(paths[i], X_OK))
		{
			path = paths[i];
			break;
		}
	char *argv[] =
	    {
	        path,
	        "-z",
	        "-u",
	        mountpoint,
	        0,
	    };
	pid_t pid = 0;
	int status = 0;
	return 0 == posix_spawn(&pid, argv[0], 0, 0, argv, 0) &&
	       pid == waitpid(pid, &status, 0) &&
	       WIFEXITED(status) && 0 == WEXITSTATUS(status);
#elif defined(_WIN32)
	// windows/winfsp: fuse_exit just works from anywhere
	fuse_exit(fuse);
	return 1;
#endif
}

static int hostNotify(struct fuse *fuse, const char *path, uint32_t action)
{
#if defined(_WIN32)
	if (0 == pfn_fsp_fuse_notify)
		return 0;
	return 0 == pfn_fsp_fuse_notify(fsp_fuse_env(), fuse, path, action);
#else
	return 0;
#endif
}

static void hostOptSet(struct fuse_opt *opt,
    const char *templ, fuse_opt_offset_t offset, int value)
{
	memset(opt, 0, sizeof *opt);
#if defined(__OpenBSD__)
	opt->templ = templ;
	opt->off = offset;
	opt->val = value;
#else
	opt->templ = templ;
	opt->offset = offset;
	opt->value = value;
#endif
}

static int hostOptParseOptProc(void *opt_data, const char *arg, int key,
    struct fuse_args *outargs)
{
	switch (key)
	{
	default:
		return 0;
	case FUSE_OPT_KEY_NONOPT:
		return 1;
	}
}

static int hostOptParse(struct fuse_args *args, void *data, const struct fuse_opt opts[],
    bool nonopts)
{
	return fuse_opt_parse(args, data, opts, nonopts ? hostOptParseOptProc : 0);
}
