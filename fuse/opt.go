/*
 * opt.go
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
	"errors"
	"runtime"
	"strings"
	"unsafe"
)

func optNormBool(opt string) string {
	if i := strings.Index(opt, "=%"); -1 != i {
		switch opt[i+2:] {
		case "d", "o", "x", "X":
			return opt
		case "v":
			return opt[:i+1]
		default:
			panic("unknown format " + opt[i+1:])
		}
	} else {
		return opt
	}
}

func optNormInt(opt string, modf string) string {
	if i := strings.Index(opt, "=%"); -1 != i {
		switch opt[i+2:] {
		case "d", "o", "x", "X":
			return opt[:i+2] + modf + opt[i+2:]
		case "v":
			return opt[:i+2] + modf + "i"
		default:
			panic("unknown format " + opt[i+1:])
		}
	} else if strings.HasSuffix(opt, "=") {
		return opt + "%" + modf + "i"
	} else {
		return opt + "=%" + modf + "i"
	}
}

func optNormStr(opt string) string {
	if i := strings.Index(opt, "=%"); -1 != i {
		switch opt[i+2:] {
		case "s", "v":
			return opt[:i+2] + "s"
		default:
			panic("unknown format " + opt[i+1:])
		}
	} else if strings.HasSuffix(opt, "=") {
		return opt + "%s"
	} else {
		return opt + "=%s"
	}
}

// OptParse parses the FUSE command line arguments in args as determined by format
// and stores the resulting values in vals, which must be pointers. It returns a
// list of unparsed arguments or nil if an error happens.
//
// The format may be empty or non-empty. An empty format is taken as a special
// instruction to OptParse to only return all non-option arguments in outargs.
//
// A non-empty format is a space separated list of acceptable FUSE options. Each
// option is matched with a corresponding pointer value in vals. The combination
// of the option and the type of the corresponding pointer value, determines how
// the option is used. The allowed pointer types are pointer to bool, pointer to
// an integer type and pointer to string.
//
// For pointer to bool types:
//
//	-x                       Match -x without parameter.
//	-foo --foo               As above for -foo or --foo.
//	foo                      Match "-o foo".
//	-x= -foo= --foo= foo=    Match option with parameter.
//	-x=%VERB ... foo=%VERB   Match option with parameter of syntax.
//	                         Allowed verbs: d,o,x,X,v
//	                         - d,o,x,X: set to true if parameter non-0.
//	                         - v: set to true if parameter present.
//
//	The formats -x=, and -x=%v are equivalent.
//
// For pointer to other types:
//
//	-x                       Match -x with parameter (-x=PARAM).
//	-foo --foo               As above for -foo or --foo.
//	foo                      Match "-o foo=PARAM".
//	-x= -foo= --foo= foo=    Match option with parameter.
//	-x=%VERB ... foo=%VERB   Match option with parameter of syntax.
//	                         Allowed verbs for pointer to int types: d,o,x,X,v
//	                         Allowed verbs for pointer to string types: s,v
//
//	The formats -x, -x=, and -x=%v are equivalent.
//
// For example:
//
//	var f bool
//	var set_attr_timeout bool
//	var attr_timeout int
//	var umask uint32
//	outargs, err := OptParse(args, "-f attr_timeout= attr_timeout umask=%o",
//	    &f, &set_attr_timeout, &attr_timeout, &umask)
//
// Will accept a command line of:
//
//	$ program -f -o attr_timeout=42,umask=077
//
// And will set variables as follows:
//
//	f == true
//	set_attr_timeout == true
//	attr_timeout == 42
//	umask == 077
func OptParse(args []string, format string, vals ...interface{}) (outargs []string, err error) {
	if 0 == c_hostFuseInit() {
		if "windows" == runtime.GOOS {
			panic("cgofuse: cannot find winfsp")
		} else {
			panic("cgofuse: cannot find FUSE")
		}
	}

	defer func() {
		if r := recover(); nil != r {
			if s, ok := r.(string); ok {
				outargs = nil
				err = errors.New("OptParse: " + s)
			} else {
				panic(r)
			}
		}
	}()

	var opts []string
	var nonopts bool
	if "" == format {
		opts = make([]string, 0)
		nonopts = true
	} else {
		opts = strings.Split(format, " ")
	}

	// Guard against a caller passing fewer vals than the format has options:
	// indexing vals[i] below would otherwise panic with index-out-of-range,
	// which the deferred recover does not convert (it only handles string
	// panics) and would re-raise into the caller.
	if len(vals) < len(opts) {
		return nil, errors.New("OptParse: format has more options than values")
	}

	align := int(2 * unsafe.Sizeof(c_size_t(0))) // match malloc alignment (usually 8 or 16)

	fuse_opts := make([]c_struct_fuse_opt, len(opts)+1)
	for i := 0; len(opts) > i; i++ {
		var templ *c_char
		switch vals[i].(type) {
		case *bool:
			templ = c_CString(optNormBool(opts[i]))
		case *int:
			templ = c_CString(optNormInt(opts[i], ""))
		case *int8:
			templ = c_CString(optNormInt(opts[i], "hh"))
		case *int16:
			templ = c_CString(optNormInt(opts[i], "h"))
		case *int32:
			templ = c_CString(optNormInt(opts[i], ""))
		case *int64:
			templ = c_CString(optNormInt(opts[i], "ll"))
		case *uint:
			templ = c_CString(optNormInt(opts[i], ""))
		case *uint8:
			templ = c_CString(optNormInt(opts[i], "hh"))
		case *uint16:
			templ = c_CString(optNormInt(opts[i], "h"))
		case *uint32:
			templ = c_CString(optNormInt(opts[i], ""))
		case *uint64:
			templ = c_CString(optNormInt(opts[i], "ll"))
		case *uintptr:
			templ = c_CString(optNormInt(opts[i], "ll"))
		case *string:
			templ = c_CString(optNormStr(opts[i]))
		default:
			return nil, errors.New("OptParse: unsupported value type for option " + opts[i])
		}
		defer c_free(unsafe.Pointer(templ))

		c_hostOptSet(&fuse_opts[i], templ, c_fuse_opt_offset_t(i*align), 1)
	}

	fuse_args := c_struct_fuse_args{}
	defer c_fuse_opt_free_args(&fuse_args)
	argc := 1 + len(args)
	argp := c_calloc(c_size_t(argc+1), c_size_t(unsafe.Sizeof((*c_char)(nil))))
	argv := (*[1 << 16]*c_char)(argp)
	argv[0] = c_CString("<UNKNOWN>")
	for i := 0; len(args) > i; i++ {
		argv[1+i] = c_CString(args[i])
	}
	fuse_args.allocated = 1
	fuse_args.argc = c_int(argc)
	fuse_args.argv = (**c_char)(&argv[0])

	data := c_calloc(c_size_t(len(opts)), c_size_t(align))
	defer c_free(data)

	if -1 == c_hostOptParse(&fuse_args, data, &fuse_opts[0], c_bool(nonopts)) {
		panic("failed")
	}

	for i := 0; len(opts) > i; i++ {
		switch v := vals[i].(type) {
		case *bool:
			*v = 0 != int(*(*c_int)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int:
			*v = int(*(*c_int)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int8:
			*v = int8(*(*c_int8_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int16:
			*v = int16(*(*c_int16_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int32:
			*v = int32(*(*c_int32_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *int64:
			*v = int64(*(*c_int64_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint:
			*v = uint(*(*c_unsigned)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint8:
			*v = uint8(*(*c_uint8_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint16:
			*v = uint16(*(*c_uint16_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint32:
			*v = uint32(*(*c_uint32_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uint64:
			*v = uint64(*(*c_uint64_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *uintptr:
			*v = uintptr(*(*c_uintptr_t)(unsafe.Pointer(uintptr(data) + uintptr(i*align))))
		case *string:
			s := *(**c_char)(unsafe.Pointer(uintptr(data) + uintptr(i*align)))
			*v = c_GoString(s)
			c_free(unsafe.Pointer(s))
		}
	}

	if 1 >= fuse_args.argc {
		outargs = make([]string, 0)
	} else {
		outargs = make([]string, fuse_args.argc-1)
		for i := 1; int(fuse_args.argc) > i; i++ {
			outargs[i-1] = c_GoString((*[1 << 16]*c_char)(unsafe.Pointer(fuse_args.argv))[i])
		}
	}

	if nonopts && 1 <= len(outargs) && "--" == outargs[0] {
		outargs = outargs[1:]
	}

	return
}
