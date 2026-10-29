package hikrobot

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"sync/atomic"
	"time"
)

var errReleased = errors.New("hikrobot: frame was released")

type frameStorage struct{ data []byte }

// Frame is one image copied out of the SDK. Its pixels are immutable and stay
// valid until the last owner calls Release. Grab hands you one ownership;
// Retain adds another, e.g. before passing the frame to another goroutine.
type Frame struct {
	Width, Height int
	PixelType     PixelType
	// Number is the camera's frame counter.
	Number uint32
	// LostPackets counts GigE packets missing from this frame.
	LostPackets uint32
	// HostTime is when the host received the frame.
	HostTime time.Time
	// DeviceTimestamp is the camera's tick counter at exposure (see the
	// GevTimestampTickFrequency node for its rate).
	DeviceTimestamp uint64

	storage *frameStorage
	pool    *sync.Pool
	refs    atomic.Int32
}

// NewFrame copies data into a standalone frame, e.g. to replay a saved raw
// image through Camera.Image.
func NewFrame(width, height int, pixelType PixelType, data []byte) (*Frame, error) {
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("hikrobot: invalid frame size %dx%d", width, height)
	}
	if need := width * height * pixelType.BitsPerPixel() / 8; len(data) < need {
		return nil, fmt.Errorf("hikrobot: short %v frame: %d bytes, need %d", pixelType, len(data), need)
	}
	f := &Frame{Width: width, Height: height, PixelType: pixelType,
		storage: &frameStorage{data: append([]byte(nil), data...)}}
	f.refs.Store(1)
	return f, nil
}

// Data returns the pixel bytes as the camera sent them. Do not modify them,
// and do not use them after your last Release.
func (f *Frame) Data() []byte { return f.storage.data }

// Retain adds an ownership. It returns false if the frame was already fully
// released, in which case the caller must not use it.
func (f *Frame) Retain() bool {
	for {
		n := f.refs.Load()
		if n <= 0 {
			return false
		}
		if f.refs.CompareAndSwap(n, n+1) {
			return true
		}
	}
}

// Release gives up one ownership. The last Release recycles the pixel buffer
// for a later Grab.
func (f *Frame) Release() {
	n := f.refs.Add(-1)
	if n < 0 {
		panic("hikrobot: Frame released more times than retained")
	}
	if n == 0 && f.pool != nil {
		f.pool.Put(f.storage)
	}
}

func grow(b []byte, size int) []byte {
	if cap(b) < size {
		return make([]byte, size)
	}
	return b[:size]
}

// previewSize keeps the aspect ratio and even dimensions, so a Bayer preview
// still holds whole 2x2 colour cells.
func previewSize(width, height, target int) (int, int, error) {
	if target <= 0 || target >= width {
		return width, height, nil
	}
	if target < 2 || width < 2 || height < 2 {
		return 0, 0, fmt.Errorf("hikrobot: preview width must be 0 or at least 2")
	}
	w := target &^ 1
	h := max(2, ((height*w+width)/(2*width))*2)
	return w, min(h, height&^1), nil
}

// downsampleBayer picks whole CFA cells, so every output 2x2 cell keeps the
// source's colour phase. Ordinary image resizing would mix colours.
func downsampleBayer(src []byte, sw, sh, dw, dh int, dst []byte) {
	for y := 0; y < dh; y += 2 {
		sy := 2 * ((y / 2) * (sh / 2) / (dh / 2))
		for x := 0; x < dw; x += 2 {
			sx := 2 * ((x / 2) * (sw / 2) / (dw / 2))
			si, di := sy*sw+sx, y*dw+x
			dst[di], dst[di+1] = src[si], src[si+1]
			dst[di+dw], dst[di+dw+1] = src[si+sw], src[si+sw+1]
		}
	}
}

func downsampleMono(src []byte, sw, sh, dw, dh int, dst []byte) {
	for y := 0; y < dh; y++ {
		row := (y * sh / dh) * sw
		for x := 0; x < dw; x++ {
			dst[y*dw+x] = src[row+x*sw/dw]
		}
	}
}

func rgbaFromRGB(rgb []byte, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for src, dst := 0, 0; dst < len(img.Pix); src, dst = src+3, dst+4 {
		img.Pix[dst], img.Pix[dst+1], img.Pix[dst+2], img.Pix[dst+3] = rgb[src], rgb[src+1], rgb[src+2], 255
	}
	return img
}
