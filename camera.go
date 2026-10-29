package hikrobot

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// Camera is an open device. Its methods are safe for concurrent use; Close
// waits for calls in progress, including a Grab blocked on its timeout.
type Camera struct {
	sdk    *SDK
	device Device

	mu       sync.RWMutex // write-locked only by Close
	handle   uintptr
	opened   bool
	grabbing bool

	pool sync.Pool // *frameStorage

	converter

	exceptionID uintptr
}

// Device is the device this camera was opened from.
func (c *Camera) Device() Device { return c.device }

// do runs fn with the handle while holding the camera open.
func (c *Camera) do(fn func(h uintptr) error) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.handle == 0 {
		return ErrClosed
	}
	return fn(c.handle)
}

func cString(s string) (*byte, error) {
	if strings.IndexByte(s, 0) >= 0 {
		return nil, fmt.Errorf("hikrobot: string %q contains NUL", s)
	}
	b := append([]byte(s), 0)
	return &b[0], nil
}

// node calls an SDK function whose first two arguments are the handle and a
// node name. Like check, it keeps pointer arguments alive and in place.
//
//go:uintptrescapes
func (c *Camera) node(fn, key string, args ...uintptr) error {
	name, err := cString(key)
	if err != nil {
		return err
	}
	err = c.do(func(h uintptr) error {
		err := c.sdk.check(fn, append([]uintptr{h, uintptr(unsafe.Pointer(name))}, args...)...)
		if e, ok := err.(*Error); ok {
			e.Op += "(" + key + ")"
		}
		return err
	})
	runtime.KeepAlive(name)
	return err
}

// Close stops grabbing and releases the camera. It is safe to call twice.
func (c *Camera) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == 0 {
		return nil
	}
	if c.exceptionID != 0 {
		exceptions.remove(c.exceptionID)
	}
	var err error
	if c.grabbing {
		err = c.sdk.check("MV_CC_StopGrabbing", c.handle)
	}
	if c.opened {
		if e := c.sdk.check("MV_CC_CloseDevice", c.handle); err == nil {
			err = e
		}
	}
	if e := c.sdk.check("MV_CC_DestroyHandle", c.handle); err == nil {
		err = e
	}
	c.handle, c.opened, c.grabbing = 0, false, false
	return err
}

// StartGrabbing starts acquisition. Configure trigger mode, pixel format and
// region first; most of those nodes are locked while grabbing.
func (c *Camera) StartGrabbing() error {
	c.mu.Lock() // also excludes Grab while the state changes
	defer c.mu.Unlock()
	if c.handle == 0 {
		return ErrClosed
	}
	if err := c.sdk.check("MV_CC_StartGrabbing", c.handle); err != nil {
		return err
	}
	c.grabbing = true
	return nil
}

// StopGrabbing stops acquisition.
func (c *Camera) StopGrabbing() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == 0 {
		return ErrClosed
	}
	if err := c.sdk.check("MV_CC_StopGrabbing", c.handle); err != nil {
		return err
	}
	c.grabbing = false
	return nil
}

// SetGrabStrategy chooses which queued frame Grab returns. Set it before
// StartGrabbing.
func (c *Camera) SetGrabStrategy(s GrabStrategy) error {
	return c.do(func(h uintptr) error { return c.sdk.check("MV_CC_SetGrabStrategy", h, uintptr(s)) })
}

// SetOutputQueueSize bounds the queue kept by GrabLatest.
func (c *Camera) SetOutputQueueSize(n int) error {
	return c.do(func(h uintptr) error { return c.sdk.check("MV_CC_SetOutputQueueSize", h, uintptr(n)) })
}

// SetImageNodeNum sets how many frame buffers the SDK allocates.
func (c *Camera) SetImageNodeNum(n int) error {
	return c.do(func(h uintptr) error { return c.sdk.check("MV_CC_SetImageNodeNum", h, uintptr(n)) })
}

// ClearImageBuffer drops frames queued in the SDK.
func (c *Camera) ClearImageBuffer() error {
	return c.do(func(h uintptr) error { return c.sdk.check("MV_CC_ClearImageBuffer", h) })
}

// Grab waits up to timeout for the next frame and copies it out of the SDK.
// It returns ErrTimeout (via errors.Is) when no frame arrived. Release the
// frame when done with it.
func (c *Camera) Grab(timeout time.Duration) (*Frame, error) {
	var f *Frame
	err := c.do(func(h uintptr) (err error) {
		var out frameOut
		ms := max(0, timeout.Milliseconds())
		if err := c.sdk.check("MV_CC_GetImageBuffer", h, uintptr(unsafe.Pointer(&out)), uintptr(ms)); err != nil {
			return err
		}
		defer func() {
			if e := c.sdk.check("MV_CC_FreeImageBuffer", h, uintptr(unsafe.Pointer(&out))); err == nil && e != nil {
				f.Release()
				f, err = nil, e
			}
		}()
		f, err = c.copyFrame(&out)
		return err
	})
	return f, err
}

func (c *Camera) copyFrame(out *frameOut) (*Frame, error) {
	fi := &out.Info
	width, height := int(fi.Width), int(fi.Height)
	if fi.ExtendedWidth != 0 || fi.ExtendedHeight != 0 {
		width, height = int(fi.ExtendedWidth), int(fi.ExtendedHeight)
	}
	size := uint64(fi.Length)
	if fi.ExtendedLength != 0 {
		size = fi.ExtendedLength
	}
	if out.Data == nil || width < 1 || height < 1 || size == 0 || size > 1<<31 {
		return nil, fmt.Errorf("hikrobot: invalid frame from SDK: %dx%d, %d bytes", width, height, size)
	}
	storage, _ := c.pool.Get().(*frameStorage)
	if storage == nil {
		storage = &frameStorage{}
	}
	storage.data = grow(storage.data, int(size))
	copy(storage.data, unsafe.Slice(out.Data, int(size)))
	f := &Frame{
		Width: width, Height: height, PixelType: PixelType(fi.PixelType),
		Number: fi.Number, LostPackets: fi.LostPackets,
		DeviceTimestamp: uint64(fi.DeviceTimeHigh)<<32 | uint64(fi.DeviceTimeLow),
		storage:         storage, pool: &c.pool,
	}
	if fi.HostTime > 0 {
		f.HostTime = time.UnixMilli(fi.HostTime)
	}
	f.refs.Store(1)
	return f, nil
}

// IntValue is an integer node's value and constraints.
type IntValue struct{ Current, Min, Max, Increment int64 }

// GetInt reads an integer node, e.g. "Width".
func (c *Camera) GetInt(key string) (IntValue, error) {
	var v intValue
	err := c.node("MV_CC_GetIntValueEx", key, uintptr(unsafe.Pointer(&v)))
	return IntValue{v.Current, v.Min, v.Max, v.Increment}, err
}

// SetInt writes an integer node.
func (c *Camera) SetInt(key string, value int64) error {
	return c.node("MV_CC_SetIntValueEx", key, uintptr(value))
}

// FloatValue is a float node's value and limits.
type FloatValue struct{ Current, Min, Max float32 }

// GetFloat reads a float node, e.g. "ExposureTime".
func (c *Camera) GetFloat(key string) (FloatValue, error) {
	var v floatValue
	err := c.node("MV_CC_GetFloatValue", key, uintptr(unsafe.Pointer(&v)))
	return FloatValue{v.Current, v.Min, v.Max}, err
}

// SetFloat writes a float node, e.g. "ExposureTime" in µs.
func (c *Camera) SetFloat(key string, value float32) error {
	name, err := cString(key)
	if err != nil {
		return err
	}
	err = c.do(func(h uintptr) error {
		r, err := c.sdk.lib.callFloat("MV_CC_SetFloatValue", h, name, value)
		if err != nil {
			return err
		}
		if code := uint32(r); code != 0 {
			return &Error{Op: "MV_CC_SetFloatValue(" + key + ")", Code: code}
		}
		return nil
	})
	runtime.KeepAlive(name)
	return err
}

// EnumValue is an enumeration node's current value and the values the camera
// accepts right now.
type EnumValue struct {
	Current   uint32
	Supported []uint32
}

// GetEnum reads an enumeration node's numeric value.
func (c *Camera) GetEnum(key string) (EnumValue, error) {
	var v enumValue
	if err := c.node("MV_CC_GetEnumValue", key, uintptr(unsafe.Pointer(&v))); err != nil {
		return EnumValue{}, err
	}
	n := min(int(v.Count), len(v.Supported))
	return EnumValue{Current: v.Current, Supported: append([]uint32(nil), v.Supported[:n]...)}, nil
}

// SetEnum writes an enumeration node by numeric value.
func (c *Camera) SetEnum(key string, value uint32) error {
	return c.node("MV_CC_SetEnumValue", key, uintptr(value))
}

// SetEnumString writes an enumeration node by symbol, e.g.
// SetEnumString("TriggerMode", "Off").
func (c *Camera) SetEnumString(key, symbol string) error {
	value, err := cString(symbol)
	if err != nil {
		return err
	}
	return c.node("MV_CC_SetEnumValueByString", key, uintptr(unsafe.Pointer(value)))
}

// EnumSymbol names one value of an enumeration node.
func (c *Camera) EnumSymbol(key string, value uint32) (string, error) {
	e := enumEntry{Value: value}
	if err := c.node("MV_CC_GetEnumEntrySymbolic", key, uintptr(unsafe.Pointer(&e))); err != nil {
		return "", err
	}
	return cstr(e.Symbol[:]), nil
}

// GetEnumString reads an enumeration node's current symbol, e.g. "BayerGB8"
// for "PixelFormat".
func (c *Camera) GetEnumString(key string) (string, error) {
	v, err := c.GetEnum(key)
	if err != nil {
		return "", err
	}
	return c.EnumSymbol(key, v.Current)
}

// GetBool reads a boolean node.
func (c *Camera) GetBool(key string) (bool, error) {
	var v byte // C++ bool
	err := c.node("MV_CC_GetBoolValue", key, uintptr(unsafe.Pointer(&v)))
	return v != 0, err
}

// SetBool writes a boolean node.
func (c *Camera) SetBool(key string, value bool) error {
	var v uintptr
	if value {
		v = 1
	}
	return c.node("MV_CC_SetBoolValue", key, v)
}

// GetString reads a string node, e.g. "DeviceUserID".
func (c *Camera) GetString(key string) (string, error) {
	var v stringValue
	err := c.node("MV_CC_GetStringValue", key, uintptr(unsafe.Pointer(&v)))
	return cstr(v.Current[:]), err
}

// SetString writes a string node.
func (c *Camera) SetString(key, value string) error {
	v, err := cString(value)
	if err != nil {
		return err
	}
	return c.node("MV_CC_SetStringValue", key, uintptr(unsafe.Pointer(v)))
}

// Execute runs a command node, e.g. "TriggerSoftware".
func (c *Camera) Execute(key string) error { return c.node("MV_CC_SetCommandValue", key) }

// SaveFeatures writes the camera's current settings to a file that
// LoadFeatures can apply later.
func (c *Camera) SaveFeatures(path string) error { return c.file("MV_CC_FeatureSave", path) }

// LoadFeatures applies settings saved by SaveFeatures. Stop grabbing first.
func (c *Camera) LoadFeatures(path string) error { return c.file("MV_CC_FeatureLoad", path) }

func (c *Camera) file(fn, path string) error {
	p, err := cString(path)
	if err != nil {
		return err
	}
	return c.do(func(h uintptr) error { return c.sdk.check(fn, h, uintptr(unsafe.Pointer(p))) })
}

// Connected reports whether the SDK still reaches the camera.
func (c *Camera) Connected() bool {
	connected := false
	_ = c.do(func(h uintptr) error {
		r, err := c.sdk.lib.call("MV_CC_IsDeviceConnected", h)
		connected = err == nil && byte(r) != 0
		return nil
	})
	return connected
}

// OptimalPacketSize is the largest GigE packet the network path carries.
func (c *Camera) OptimalPacketSize() (int, error) {
	size := 0
	err := c.do(func(h uintptr) error {
		r, err := c.sdk.lib.call("MV_CC_GetOptimalPacketSize", h)
		if err != nil {
			return err
		}
		if int32(r) <= 0 {
			return &Error{Op: "MV_CC_GetOptimalPacketSize", Code: uint32(r)}
		}
		size = int(int32(r))
		return nil
	})
	return size, err
}

// SetResend controls GigE packet resend. maxPercent caps resend requests as a
// share of a frame's packets; timeout is how long to wait for a resent packet.
func (c *Camera) SetResend(enable bool, maxPercent int, timeout time.Duration) error {
	var on uintptr
	if enable {
		on = 1
	}
	return c.do(func(h uintptr) error {
		return c.sdk.check("MV_GIGE_SetResend", h, on, uintptr(maxPercent), uintptr(timeout.Milliseconds()))
	})
}

// OnException calls fn, on its own goroutine, for each asynchronous device
// event such as ExceptionDisconnected. A later call replaces fn.
func (c *Camera) OnException(fn func(Exception)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == 0 {
		return ErrClosed
	}
	if c.exceptionID == 0 {
		c.exceptionID = exceptions.add(fn)
		if err := c.sdk.check("MV_CC_RegisterExceptionCallBack", c.handle, exceptions.callback(), c.exceptionID); err != nil {
			exceptions.remove(c.exceptionID)
			c.exceptionID = 0
			return err
		}
		return nil
	}
	exceptions.set(c.exceptionID, fn)
	return nil
}

// exceptions routes the single SDK callback to each camera's handler. The
// callback is created once per process because Windows callbacks are never
// freed.
var exceptions = &exceptionRegistry{handlers: make(map[uintptr]func(Exception))}

type exceptionRegistry struct {
	once     sync.Once
	ptr      uintptr
	mu       sync.Mutex
	next     uintptr
	handlers map[uintptr]func(Exception)
}

func (r *exceptionRegistry) callback() uintptr {
	r.once.Do(func() { r.ptr = newExceptionCallback(r.dispatch) })
	return r.ptr
}

func (r *exceptionRegistry) dispatch(msgType, user uintptr) uintptr {
	r.mu.Lock()
	fn := r.handlers[user]
	r.mu.Unlock()
	if fn != nil {
		// Off the SDK's thread, so fn may call back into the SDK or Close.
		go fn(Exception(uint32(msgType)))
	}
	return 0
}

func (r *exceptionRegistry) add(fn func(Exception)) uintptr {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	r.handlers[r.next] = fn
	return r.next
}

func (r *exceptionRegistry) set(id uintptr, fn func(Exception)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[id] = fn
}

func (r *exceptionRegistry) remove(id uintptr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, id)
}
