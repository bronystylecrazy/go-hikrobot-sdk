//go:build windows && amd64

package hikrobot

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// DefaultRuntimePath is where the MVS installer puts the 64-bit runtime.
func DefaultRuntimePath() string {
	root := os.Getenv("ProgramFiles(x86)")
	if root == "" {
		root = `C:\Program Files (x86)`
	}
	return filepath.Join(root, "Common Files", "MVS", "Runtime", "Win64_x64", "MvCameraControl.dll")
}

type library struct {
	dll   *syscall.DLL
	mu    sync.Mutex
	procs map[string]*syscall.Proc
}

func openLibrary(path string) (*library, error) {
	if path == "" {
		path = DefaultRuntimePath()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// MvCameraControl.dll loads its sibling DLLs through the search path.
	if err := os.Setenv("PATH", filepath.Dir(abs)+";"+os.Getenv("PATH")); err != nil {
		return nil, err
	}
	dll, err := syscall.LoadDLL(abs)
	if err != nil {
		return nil, fmt.Errorf("hikrobot: load MVS runtime %s (is MVS installed?): %w", abs, err)
	}
	return &library{dll: dll, procs: make(map[string]*syscall.Proc)}, nil
}

// proc resolves lazily, so a runtime that lacks a newer function still loads
// and fails only when that function is used.
func (l *library) proc(name string) (*syscall.Proc, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if p, ok := l.procs[name]; ok {
		return p, nil
	}
	p, err := l.dll.FindProc(name)
	if err != nil {
		return nil, fmt.Errorf("hikrobot: %s is missing from this MVS runtime: %w", name, err)
	}
	l.procs[name] = p
	return p, nil
}

func (l *library) has(name string) bool {
	_, err := l.proc(name)
	return err == nil
}

// call returns the raw result. Pointer arguments must be converted to uintptr
// in the argument list itself so that uintptrescapes keeps them alive.
//
//go:uintptrescapes
func (l *library) call(name string, args ...uintptr) (uintptr, error) {
	p, err := l.proc(name)
	if err != nil {
		return 0, err
	}
	r, _, _ := p.Call(args...)
	return r, nil
}

func (l *library) close() { _ = l.dll.Release() }

// The Windows x64 calling convention ignores __stdcall, so one Go function
// serves every MvExceptionCallback.
func newExceptionCallback(fn func(msgType, user uintptr) uintptr) uintptr {
	return syscall.NewCallback(fn)
}
