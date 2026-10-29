package hikrobot

import (
	"fmt"
	"image"
	"sync"
	"unsafe"
)

// converter holds the SDK conversion methods shared by Camera and Converter.
type converter struct {
	sdk *SDK
	use func(fn func(h uintptr) error) error // runs fn with a live handle

	mu         sync.Mutex // guards the scratch buffers below
	small, rgb []byte
}

// SetBayerQuality sets the demosaicing used by Image, Preview and ConvertTo.
func (c *converter) SetBayerQuality(q BayerQuality) error {
	return c.use(func(h uintptr) error { return c.sdk.check("MV_CC_SetBayerCvtQuality", h, uintptr(q)) })
}

// ConvertTo converts f to the dst pixel type (e.g. BGR8Packed for OpenCV)
// with the SDK, reusing buf when it is large enough.
func (c *converter) ConvertTo(f *Frame, dst PixelType, buf []byte) ([]byte, error) {
	if !f.Retain() {
		return nil, errReleased
	}
	defer f.Release()
	buf = grow(buf, f.Width*f.Height*dst.BitsPerPixel()/8)
	return buf, c.convertPixels(f.Data(), f.Width, f.Height, f.PixelType, dst, buf)
}

// Image converts f to full-resolution RGBA.
func (c *converter) Image(f *Frame) (*image.RGBA, error) { return c.Preview(f, 0) }

// Preview converts f to RGBA scaled to width pixels wide (0 keeps the full
// size). Mono8 and 8-bit Bayer frames are downsampled before demosaicing,
// keeping whole colour cells, which makes a small live view much cheaper than
// converting the full frame.
func (c *converter) Preview(f *Frame, width int) (*image.RGBA, error) {
	if !f.Retain() {
		return nil, errReleased
	}
	defer f.Release()
	w, h, err := previewSize(f.Width, f.Height, width)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	src := f.Data()
	if w != f.Width || h != f.Height {
		if !f.PixelType.Is8BitRaw() {
			return nil, fmt.Errorf("hikrobot: preview scaling needs Mono8 or 8-bit Bayer, got %v", f.PixelType)
		}
		c.small = grow(c.small, w*h)
		if f.PixelType == Mono8 {
			downsampleMono(src, f.Width, f.Height, w, h, c.small)
		} else {
			downsampleBayer(src, f.Width, f.Height, w, h, c.small)
		}
		src = c.small
	}
	if f.PixelType == RGB8Packed {
		return rgbaFromRGB(src, w, h), nil
	}
	c.rgb = grow(c.rgb, w*h*3)
	if err := c.convertPixels(src, w, h, f.PixelType, RGB8Packed, c.rgb); err != nil {
		return nil, err
	}
	return rgbaFromRGB(c.rgb, w, h), nil
}

func (c *converter) convertPixels(src []byte, width, height int, from, to PixelType, dst []byte) error {
	if len(src) == 0 || len(dst) == 0 {
		return fmt.Errorf("hikrobot: empty conversion buffer")
	}
	p := pixelConvert{Width: uint32(width), Height: uint32(height), SourcePixelType: pixelEnum(from),
		Source: &src[0], SourceLength: uint32(len(src)), DestinationPixelType: pixelEnum(to),
		Destination: &dst[0], DestinationSize: uint32(len(dst))}
	err := c.use(func(h uintptr) error {
		return c.sdk.check("MV_CC_ConvertPixelTypeEx", h, uintptr(unsafe.Pointer(&p)))
	})
	if err != nil {
		return err
	}
	if int(p.DestinationLength) != len(dst) {
		return fmt.Errorf("hikrobot: SDK converted %d bytes, expected %d", p.DestinationLength, len(dst))
	}
	return nil
}

// Converter runs the SDK's pixel conversion without a camera, e.g. on raw
// frames saved earlier and loaded with NewFrame. It is safe for concurrent
// use.
type Converter struct {
	converter

	mu     sync.RWMutex // write-locked only by Close
	handle uintptr
}

// NewConverter creates a conversion handle that is not tied to any camera.
// Close it when done.
func (s *SDK) NewConverter() (*Converter, error) {
	// The SDK needs device information to create a handle but only contacts
	// the device on open, so a placeholder GigE entry suffices.
	var info deviceInfo
	info.Transport = uint32(TransportGigE)
	copy(info.Special[52:84], "go-hikrobot-sdk converter")
	create := "MV_CC_CreateHandleWithoutLog"
	if !s.lib.has(create) {
		create = "MV_CC_CreateHandle"
	}
	k := &Converter{}
	if err := s.check(create, uintptr(unsafe.Pointer(&k.handle)), uintptr(unsafe.Pointer(&info))); err != nil {
		return nil, err
	}
	k.converter = converter{sdk: s, use: k.do}
	return k, nil
}

func (k *Converter) do(fn func(h uintptr) error) error {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.handle == 0 {
		return ErrClosed
	}
	return fn(k.handle)
}

// Close releases the handle. It is safe to call twice.
func (k *Converter) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.handle == 0 {
		return nil
	}
	err := k.sdk.check("MV_CC_DestroyHandle", k.handle)
	k.handle = 0
	return err
}
