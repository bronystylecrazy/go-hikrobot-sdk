//go:build !(windows && amd64) && !((linux || darwin) && (amd64 || arm64))

package hikrobot

// DefaultRuntimePath is empty: MVS has no runtime this package can load on
// this platform.
func DefaultRuntimePath() string { return "" }

type library struct{}

func openLibrary(string) (*library, error)                { return nil, ErrUnsupported }
func (*library) has(string) bool                          { return false }
func (*library) call(string, ...uintptr) (uintptr, error) { return 0, ErrUnsupported }
func (*library) callFloat(string, uintptr, *byte, float32) (uintptr, error) {
	return 0, ErrUnsupported
}
func (*library) close()                                           {}
func newExceptionCallback(func(uintptr, uintptr) uintptr) uintptr { return 0 }
