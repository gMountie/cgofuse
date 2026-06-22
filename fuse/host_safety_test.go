/*
 * host_safety_test.go
 *
 * Error-path tests for the panic->errno net, the errno string table, and
 * OptParse caller-error handling. Pure Go (no mount), so they run under every
 * build variant.
 */

package fuse

import "testing"

func TestRecoverAsErrno(t *testing.T) {
	fromPanic := func(v interface{}) c_int {
		var errc c_int
		func() {
			defer recoverAsErrno(&errc)
			panic(v)
		}()
		return errc
	}
	if got := fromPanic(Error(-ENOENT)); got != c_int(-ENOENT) {
		t.Errorf("Error panic: got %d, want %d", got, -ENOENT)
	}
	if got := fromPanic("boom"); got != -c_int(EIO) {
		t.Errorf("non-Error panic: got %d, want %d", got, -EIO)
	}
	// A normal return must leave the value untouched.
	errc := c_int(3)
	func() { defer recoverAsErrno(&errc) }()
	if errc != 3 {
		t.Errorf("no panic: got %d, want 3", errc)
	}
}

func TestRecoverAsErrno64(t *testing.T) {
	fromPanic := func(v interface{}) c_fuse_off_t {
		var rslt c_fuse_off_t
		func() {
			defer recoverAsErrno64(&rslt)
			panic(v)
		}()
		return rslt
	}
	if got := fromPanic(Error(-ENOENT)); got != c_fuse_off_t(-ENOENT) {
		t.Errorf("Error panic: got %d, want %d", got, -ENOENT)
	}
	if got := fromPanic("boom"); got != -c_fuse_off_t(EIO) {
		t.Errorf("non-Error panic: got %d, want %d", got, -EIO)
	}
}

func TestErrorString(t *testing.T) {
	if s := Error(-ENOENT).Error(); s != "-fuse.ENOENT" {
		t.Errorf("Error(-ENOENT) = %q, want -fuse.ENOENT", s)
	}
	if s := Error(5).Error(); s != "5" {
		t.Errorf("Error(5) = %q, want 5", s)
	}
	if s := Error(-7777777).Error(); s != "fuse.Error(-7777777)" {
		t.Errorf("unknown errno = %q, want fuse.Error(-7777777)", s)
	}
}

func TestOptParseErrors(t *testing.T) {
	var b bool
	// More format options than values used to index out of range and crash the
	// program; it must return an error instead.
	if _, err := OptParse(nil, "-x -y", &b); nil == err {
		t.Error("expected error when format has more options than values")
	}
	// An unsupported value type used to leave a nil template; it must error.
	var ch chan int
	if _, err := OptParse(nil, "-x", &ch); nil == err {
		t.Error("expected error for unsupported value type")
	}
}
