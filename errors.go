package hikrobot

import (
	"errors"
	"fmt"
)

// ErrUnsupported is returned by Load on platforms without the MVS runtime
// binding. Capture currently requires windows/amd64.
var ErrUnsupported = errors.New("hikrobot: MVS capture requires windows/amd64")

// ErrClosed is returned by calls on a closed Camera.
var ErrClosed = errors.New("hikrobot: camera is closed")

// Error is a failed MVS call. Code is the SDK's MV_E_* value; Op names the
// SDK function that returned it.
type Error struct {
	Op   string
	Code uint32
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("0x%08X", e.Code)
	if name, ok := errorNames[e.Code]; ok {
		msg = name + " (" + msg + ")"
	}
	if e.Op == "" {
		return "hikrobot: " + msg
	}
	return "hikrobot: " + e.Op + ": " + msg
}

// Is matches any Error with the same Code, so errors.Is(err, ErrTimeout)
// holds whichever SDK function reported it.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// Common SDK failures, for use with errors.Is.
var (
	ErrTimeout         = &Error{Code: 0x80000007} // MV_E_NODATA: no frame within the timeout
	ErrNotSupported    = &Error{Code: 0x80000001} // MV_E_SUPPORT
	ErrParameter       = &Error{Code: 0x80000004} // MV_E_PARAMETER
	ErrNodeNotFound    = &Error{Code: 0x80000109} // MV_E_GC_NODE_NOT_FOUND
	ErrNodeAccess      = &Error{Code: 0x80000106} // MV_E_GC_ACCESS: node not readable/writable now
	ErrAccessDenied    = &Error{Code: 0x80000203} // MV_E_ACCESS_DENIED: opened by another program
	ErrBusy            = &Error{Code: 0x80000204} // MV_E_BUSY: device busy or network down
	ErrDisconnected    = &Error{Code: 0x80000211} // MV_E_DEV_DISCONNECT
	ErrBufferTooSmall  = &Error{Code: 0x8000000A} // MV_E_NOENOUGH_BUF
	ErrUnsupportedType = &Error{Code: 0x80000500} // MV_E_SUPPORT_PIXEL_FORMAT
)

// errorNames covers the codes an application is likely to meet; the full list
// is in the SDK's MvErrorDefine.h.
var errorNames = map[uint32]string{
	0x80000000: "invalid handle",
	0x80000001: "not supported",
	0x80000002: "buffer overflow",
	0x80000003: "wrong call order",
	0x80000004: "invalid parameter",
	0x80000006: "out of resources",
	0x80000007: "no data (timeout)",
	0x80000008: "precondition failed",
	0x80000009: "version mismatch",
	0x8000000A: "buffer too small",
	0x8000000B: "abnormal image (lost packets)",
	0x8000000C: "failed to load library",
	0x8000000D: "no output buffer",
	0x8000000F: "cannot open file",
	0x80000020: "device offline",
	0x80000025: "parameter out of range",
	0x80000028: "resource in use",
	0x800000FE: "SDK internal error",
	0x800000FF: "unknown error",
	0x80000100: "GenICam error",
	0x80000101: "GenICam: invalid argument",
	0x80000102: "GenICam: value out of range",
	0x80000103: "GenICam: property error",
	0x80000104: "GenICam: runtime error",
	0x80000105: "GenICam: logical error",
	0x80000106: "GenICam: node not accessible",
	0x80000107: "GenICam: timeout",
	0x80000108: "GenICam: type mismatch",
	0x80000109: "GenICam: node not found",
	0x8000010A: "GenICam: node validation failed",
	0x8000010B: "GenICam: file error",
	0x800001FF: "GenICam: unknown error",
	0x80000200: "command not implemented by device",
	0x80000201: "invalid address",
	0x80000202: "write protected",
	0x80000203: "access denied (device open in another program?)",
	0x80000204: "device busy or network disconnected",
	0x80000205: "network packet error",
	0x80000206: "network error",
	0x80000207: "GigE driver not attached",
	0x8000020C: "network timeout",
	0x80000211: "device disconnected",
	0x80000221: "IP conflict",
	0x80000300: "USB read error",
	0x80000301: "USB write error",
	0x80000302: "USB device error",
	0x80000303: "USB GenICam error",
	0x80000304: "insufficient USB bandwidth",
	0x80000305: "USB driver mismatch or missing",
	0x800003FF: "USB unknown error",
	0x80000500: "unsupported pixel format",
	0x80000502: "insufficient input image data",
}
