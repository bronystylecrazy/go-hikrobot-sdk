//go:build !windows

package hikrobot

// pixelEnum mirrors a C enum MvGvspPixelType field. GCC and Clang make it 8
// bytes, because the enum holds both -1 and values above 0x7FFFFFFF; the MVS
// Linux runtime is built that way (checked against MVS 5.1 for aarch64).
type pixelEnum = uint64
