/*
 * host_handle_test.go
 *
 * Tests and benchmarks for the FileSystemHost handle table. These exercise the
 * build-specific hostHandleNew/Get/Del directly, without a FUSE mount, so they
 * run anywhere (FUSE mount tests require /dev/fuse and a real mount point).
 */

package fuse

import (
	"runtime"
	"sync"
	"testing"
	"unsafe"
)

// TestHandleRoundTrip verifies a handle resolves back to the same host, and
// that forcing the garbage collector between creation and lookup is safe even
// though the cgo build stores the handle in an unsafe.Pointer-typed value.
func TestHandleRoundTrip(t *testing.T) {
	hostA := &FileSystemHost{}
	hostB := &FileSystemHost{}

	pa := hostHandleNew(hostA)
	defer hostHandleDel(pa)
	pb := hostHandleNew(hostB)
	defer hostHandleDel(pb)

	if pa == pb {
		t.Fatalf("distinct hosts produced the same handle %v", pa)
	}

	// Force several GC cycles: the handle value lives in an unsafe.Pointer
	// local, so a precise stack scan must tolerate it.
	for i := 0; i < 8; i++ {
		runtime.GC()
	}

	if got := hostHandleGet(pa); got != hostA {
		t.Fatalf("hostHandleGet(pa) = %p, want %p", got, hostA)
	}
	if got := hostHandleGet(pb); got != hostB {
		t.Fatalf("hostHandleGet(pb) = %p, want %p", got, hostB)
	}
}

// TestHandleConcurrent hammers hostHandleGet from many goroutines while the GC
// runs, mirroring the multithreaded FUSE loop that calls back per operation.
func TestHandleConcurrent(t *testing.T) {
	const goroutines = 64
	host := &FileSystemHost{}
	p := hostHandleNew(host)
	defer hostHandleDel(p)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				runtime.GC()
			}
		}
	}()

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50000; i++ {
				if hostHandleGet(p) != host {
					t.Errorf("hostHandleGet returned wrong host")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(done)
}

// --- Benchmarks ------------------------------------------------------------
//
// BenchmarkHandleGet measures the production hostHandleGet (cgo.Handle on the
// cgo build). BenchmarkHandleGetMutexMap measures the previous map+global-mutex
// design, reimplemented locally, for an apples-to-apples comparison. Run with
//   go test -run=^$ -bench=HandleGet -benchmem -cpu=1,4,8 ./fuse
// to see how each scales as the number of concurrent callers grows.

var (
	benchMutexGuard sync.Mutex
	benchMutexTable = map[unsafe.Pointer]*FileSystemHost{}
)

func benchMutexGet(p unsafe.Pointer) *FileSystemHost {
	benchMutexGuard.Lock()
	host := benchMutexTable[p]
	benchMutexGuard.Unlock()
	return host
}

func BenchmarkHandleGet(b *testing.B) {
	host := &FileSystemHost{}
	p := hostHandleNew(host)
	defer hostHandleDel(p)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if hostHandleGet(p) != host {
				b.Fatal("wrong host")
			}
		}
	})
}

func BenchmarkHandleGetMutexMap(b *testing.B) {
	host := &FileSystemHost{}
	// Use the host pointer itself as the map key, matching how the old code
	// keyed the table by an opaque unsafe.Pointer.
	p := unsafe.Pointer(host)
	benchMutexGuard.Lock()
	benchMutexTable[p] = host
	benchMutexGuard.Unlock()
	defer func() {
		benchMutexGuard.Lock()
		delete(benchMutexTable, p)
		benchMutexGuard.Unlock()
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if benchMutexGet(p) != host {
				b.Fatal("wrong host")
			}
		}
	})
}
