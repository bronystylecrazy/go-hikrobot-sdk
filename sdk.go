package hikrobot

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

// SDK is a loaded MVS runtime. One per process is enough; it is safe for
// concurrent use.
type SDK struct {
	lib      *library
	mu       sync.Mutex
	finalize bool
	closed   bool
}

// Load opens MvCameraControl.dll at path, or at DefaultRuntimePath when path
// is empty, and initializes it. Close it when done.
func Load(path string) (*SDK, error) {
	lib, err := openLibrary(path)
	if err != nil {
		return nil, err
	}
	s := &SDK{lib: lib}
	// MV_CC_Initialize arrived in MVS 4.1; older runtimes initialize on load.
	if lib.has("MV_CC_Initialize") {
		if err := s.check("MV_CC_Initialize"); err != nil {
			lib.close()
			return nil, err
		}
		s.finalize = true
	}
	return s, nil
}

// check runs an SDK function that returns an MV_E_* code.
//
//go:uintptrescapes
func (s *SDK) check(name string, args ...uintptr) error {
	r, err := s.lib.call(name, args...)
	if err != nil {
		return err
	}
	if code := uint32(r); code != 0 {
		return &Error{Op: name, Code: code}
	}
	return nil
}

// Close finalizes and unloads the runtime. Close every Camera first.
func (s *SDK) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.finalize {
		err = s.check("MV_CC_Finalize")
	}
	s.lib.close()
	return err
}

// Version is the runtime's version, e.g. "4.8.2.2".
func (s *SDK) Version() string {
	v, err := s.lib.call("MV_CC_GetSDKVersion")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.%d", byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// Devices lists the cameras reachable over the given transports, or over
// GigE and USB3 when transports is 0. Device order follows the SDK.
func (s *SDK) Devices(transports Transport) ([]Device, error) {
	if transports == 0 {
		transports = TransportGigE | TransportUSB3
	}
	list := new(deviceList)
	if err := s.check("MV_CC_EnumDevices", uintptr(transports), uintptr(unsafe.Pointer(list))); err != nil {
		return nil, err
	}
	if list.Count > uint32(len(list.Devices)) {
		return nil, fmt.Errorf("hikrobot: SDK reported %d devices", list.Count)
	}
	devices := make([]Device, 0, list.Count)
	for _, info := range list.Devices[:list.Count] {
		if info != nil {
			devices = append(devices, parseDevice(info))
		}
	}
	return devices, nil
}

// Find returns the first device whose serial number, IP address, model or
// user-defined name equals key, ignoring case.
func (s *SDK) Find(key string) (Device, error) {
	devices, err := s.Devices(0)
	if err != nil {
		return Device{}, err
	}
	for _, d := range devices {
		ip := ""
		if d.IP.IsValid() {
			ip = d.IP.String()
		}
		for _, v := range []string{d.Serial, ip, d.Model, d.UserName} {
			if v != "" && strings.EqualFold(v, key) {
				return d, nil
			}
		}
	}
	return Device{}, fmt.Errorf("hikrobot: no camera matches %q among %d found", key, len(devices))
}

// Accessible reports whether d can be opened in mode right now. It is false
// while another program, such as the MVS client, holds the camera.
func (s *SDK) Accessible(d Device, mode AccessMode) bool {
	if d.info.Transport == 0 {
		return false
	}
	r, err := s.lib.call("MV_CC_IsDeviceAccessible", uintptr(unsafe.Pointer(&d.info)), uintptr(mode))
	return err == nil && byte(r) != 0
}

// Open claims d with exclusive access. See OpenMode.
func (s *SDK) Open(d Device) (*Camera, error) { return s.OpenMode(d, AccessExclusive) }

// OpenMode opens d. Unless mode is AccessMonitor, a GigE camera also gets the
// SDK's optimal packet size for the link, as MVS's own samples do. Grabbing
// does not start until StartGrabbing.
func (s *SDK) OpenMode(d Device, mode AccessMode) (_ *Camera, err error) {
	if d.info.Transport == 0 {
		return nil, fmt.Errorf("hikrobot: device was not returned by Devices")
	}
	c := &Camera{sdk: s, device: d}
	c.converter = converter{sdk: s, use: c.do}
	if err = s.check("MV_CC_CreateHandle", uintptr(unsafe.Pointer(&c.handle)), uintptr(unsafe.Pointer(&d.info))); err != nil {
		return nil, err
	}
	if err = s.check("MV_CC_OpenDevice", c.handle, uintptr(mode), 0); err != nil {
		_ = s.check("MV_CC_DestroyHandle", c.handle)
		return nil, err
	}
	c.opened = true
	if mode != AccessMonitor && (d.Transport == TransportGigE || d.Transport == TransportGenTLGigE) {
		if size, err := c.OptimalPacketSize(); err == nil && size > 0 {
			_ = c.SetInt("GevSCPSPacketSize", int64(size))
		}
	}
	return c, nil
}
