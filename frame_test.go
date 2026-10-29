package hikrobot

import (
	"bytes"
	"sync"
	"testing"
)

func TestFrameRecyclesOnlyAfterLastRelease(t *testing.T) {
	storage := &frameStorage{data: []byte{1, 2, 3, 4}}
	pool := &sync.Pool{}
	f := &Frame{Width: 2, Height: 2, PixelType: BayerGB8, storage: storage, pool: pool}
	f.refs.Store(1)
	if !f.Retain() {
		t.Fatal("could not retain frame")
	}
	f.Release()
	if pool.Get() != nil {
		t.Fatal("recycled while another owner holds it")
	}
	f.Release()
	if got := pool.Get(); got != nil && got != storage {
		t.Fatal("storage was not returned to the pool")
	}
	if f.Retain() {
		t.Fatal("released frame was resurrected")
	}
}

func TestOverReleasePanics(t *testing.T) {
	f, _ := NewFrame(1, 1, Mono8, []byte{0})
	f.Release()
	defer func() {
		if recover() == nil {
			t.Fatal("second Release did not panic")
		}
	}()
	f.Release()
}

func TestNewFrameCopiesAndValidates(t *testing.T) {
	src := []byte{1, 2, 3, 4}
	f, err := NewFrame(2, 2, BayerGB8, src)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Release()
	src[0] = 99
	if f.Data()[0] != 1 {
		t.Fatal("frame aliases input memory")
	}
	if _, err := NewFrame(2, 2, BayerGB8, src[:3]); err == nil {
		t.Fatal("accepted a truncated frame")
	}
	if _, err := NewFrame(2, 2, RGB8Packed, make([]byte, 11)); err == nil {
		t.Fatal("accepted a truncated RGB frame")
	}
}

func TestDownsampleKeepsBayerPhase(t *testing.T) {
	const sw, sh, dw, dh = 12, 8, 6, 4
	src := make([]byte, sw*sh)
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			// Low two bits record the CFA phase; the rest the cell.
			src[y*sw+x] = byte((y/2*6+x/2)*4 + y%2*2 + x%2)
		}
	}
	orig := append([]byte(nil), src...)
	dst := make([]byte, dw*dh)
	downsampleBayer(src, sw, sh, dw, dh, dst)
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			if int(dst[y*dw+x])%4 != y%2*2+x%2 {
				t.Fatalf("mixed CFA phase at %d,%d", x, y)
			}
		}
	}
	if !bytes.Equal(src, orig) {
		t.Fatal("downsampling modified the source")
	}
}

func TestDownsampleMono(t *testing.T) {
	src := []byte{
		0, 1, 2, 3,
		4, 5, 6, 7,
		8, 9, 10, 11,
		12, 13, 14, 15,
	}
	dst := make([]byte, 4)
	downsampleMono(src, 4, 4, 2, 2, dst)
	if !bytes.Equal(dst, []byte{0, 2, 8, 10}) {
		t.Fatalf("got %v", dst)
	}
}

func TestPreviewSize(t *testing.T) {
	cases := []struct{ w, h, target, ww, wh int }{
		{5472, 3648, 1620, 1620, 1080},
		{5472, 3648, 0, 5472, 3648},
		{5472, 3648, 9000, 5472, 3648},
		{100, 50, 33, 32, 16},
	}
	for _, c := range cases {
		w, h, err := previewSize(c.w, c.h, c.target)
		if err != nil || w != c.ww || h != c.wh {
			t.Errorf("%dx%d→%d: got %dx%d, %v", c.w, c.h, c.target, w, h, err)
		}
	}
	if _, _, err := previewSize(100, 50, 1); err == nil {
		t.Error("accepted width 1")
	}
}

func TestRGBAFromRGB(t *testing.T) {
	img := rgbaFromRGB([]byte{1, 2, 3, 4, 5, 6}, 2, 1)
	if !bytes.Equal(img.Pix, []byte{1, 2, 3, 255, 4, 5, 6, 255}) {
		t.Fatalf("got %v", img.Pix)
	}
}

func BenchmarkBayerPreview1080(b *testing.B) {
	src := make([]byte, 5472*3648)
	dst := make([]byte, 1620*1080)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		downsampleBayer(src, 5472, 3648, 1620, 1080, dst)
	}
}
