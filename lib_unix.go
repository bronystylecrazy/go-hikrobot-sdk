//go:build (linux || darwin) && (amd64 || arm64)

package hikrobot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// DefaultRuntimePath is where the MVS installer puts the runtime: under
// $MVCAM_COMMON_RUNENV (set by the installer) or /opt/MVS/lib on Linux. On
// macOS it is the bare library name, resolved by the dynamic loader.
func DefaultRuntimePath() string {
	if runtime.GOOS == "darwin" {
		return "libMvCameraControl.dylib"
	}
	root := os.Getenv("MVCAM_COMMON_RUNENV")
	if root == "" {
		root = "/opt/MVS/lib"
	}
	arch := "64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}
	return filepath.Join(root, arch, "libMvCameraControl.so")
}

type library struct {
	handle uintptr
	mu     sync.Mutex
	procs  map[string]uintptr
	// setFloat is bound with RegisterFunc: SyscallN cannot pass a float
	// after integer arguments under the SysV and AAPCS64 conventions.
	setFloat func(handle uintptr, key unsafe.Pointer, value float32) int32
}

func openLibrary(path string) (*library, error) {
	candidates := []string{path}
	if path == "" {
		// Fall back to the loader's own search (LD_LIBRARY_PATH, ld.so.cache,
		// DYLD_LIBRARY_PATH) when the default install path is absent.
		candidates = []string{DefaultRuntimePath(), "libMvCameraControl.so", "libMvCameraControl.dylib"}
		if runtime.GOOS == "darwin" {
			candidates = candidates[2:]
		}
	}
	var errs []error
	for _, p := range candidates {
		h, err := purego.Dlopen(p, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			return &library{handle: h, procs: make(map[string]uintptr)}, nil
		}
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("hikrobot: load MVS runtime (is MVS installed, and its lib directory on LD_LIBRARY_PATH?): %w", errors.Join(errs...))
}

func (l *library) proc(name string) (uintptr, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if p, ok := l.procs[name]; ok {
		return p, nil
	}
	p, err := purego.Dlsym(l.handle, name)
	if err != nil {
		return 0, fmt.Errorf("hikrobot: %s is missing from this MVS runtime: %w", name, err)
	}
	l.procs[name] = p
	return p, nil
}

func (l *library) has(name string) bool {
	_, err := l.proc(name)
	return err == nil
}

// call passes integer and pointer arguments only; see callFloat.
//
//go:uintptrescapes
func (l *library) call(name string, args ...uintptr) (uintptr, error) {
	p, err := l.proc(name)
	if err != nil {
		return 0, err
	}
	r, _, _ := purego.SyscallN(p, args...)
	return r, nil
}

// callFloat calls MV_CC_SetFloatValue(handle, key, value).
func (l *library) callFloat(name string, handle uintptr, key *byte, value float32) (uintptr, error) {
	p, err := l.proc(name)
	if err != nil {
		return 0, err
	}
	l.mu.Lock()
	if l.setFloat == nil {
		purego.RegisterFunc(&l.setFloat, p)
	}
	fn := l.setFloat
	l.mu.Unlock()
	return uintptr(uint32(fn(handle, unsafe.Pointer(key), value))), nil
}

func (l *library) close() { _ = purego.Dlclose(l.handle) }

func newExceptionCallback(fn func(msgType, user uintptr) uintptr) uintptr {
	return purego.NewCallback(fn)
}
