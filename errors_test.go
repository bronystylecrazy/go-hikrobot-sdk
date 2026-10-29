package hikrobot

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorMatchesByCode(t *testing.T) {
	err := fmt.Errorf("grab: %w", &Error{Op: "MV_CC_GetImageBuffer", Code: 0x80000007})
	if !errors.Is(err, ErrTimeout) {
		t.Fatal("timeout not matched")
	}
	if errors.Is(err, ErrAccessDenied) {
		t.Fatal("matched the wrong code")
	}
	want := "hikrobot: MV_CC_GetImageBuffer: no data (timeout) (0x80000007)"
	if got := errors.Unwrap(err).Error(); got != want {
		t.Fatalf("got %q", got)
	}
	if got := (&Error{Code: 0x80001234}).Error(); got != "hikrobot: 0x80001234" {
		t.Fatalf("got %q", got)
	}
}

func TestPixelType(t *testing.T) {
	cases := []struct {
		p    PixelType
		name string
		bits int
		raw8 bool
	}{
		{Mono8, "Mono8", 8, true},
		{BayerGB8, "BayerGB8", 8, true},
		{BayerRG12, "BayerRG12", 16, false},
		{RGB8Packed, "RGB8Packed", 24, false},
		{BGRA8Packed, "BGRA8Packed", 32, false},
		{PixelType(0x01080099), "0x01080099", 8, false},
	}
	for _, c := range cases {
		if c.p.String() != c.name || c.p.BitsPerPixel() != c.bits || c.p.Is8BitRaw() != c.raw8 {
			t.Errorf("%v: got %q %d %v", uint32(c.p), c.p.String(), c.p.BitsPerPixel(), c.p.Is8BitRaw())
		}
	}
}

func TestUnsupportedPlatformOrMissingRuntime(t *testing.T) {
	if _, err := Load("definitely-missing/MvCameraControl.dll"); err == nil {
		t.Fatal("loaded a missing runtime")
	}
}
