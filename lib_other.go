//go:build !windows || !amd64

package hikrobot

// DefaultRuntimePath is where the MVS installer puts the 64-bit Windows
// runtime. Load does not use it on this platform.
func DefaultRuntimePath() string {
	return `C:\Program Files (x86)\Common Files\MVS\Runtime\Win64_x64\MvCameraControl.dll`
}

type library struct{}

func openLibrary(string) (*library, error)                        { return nil, ErrUnsupported }
func (*library) has(string) bool                                  { return false }
func (*library) call(string, ...uintptr) (uintptr, error)         { return 0, ErrUnsupported }
func (*library) close()                                           {}
func newExceptionCallback(func(uintptr, uintptr) uintptr) uintptr { return 0 }
