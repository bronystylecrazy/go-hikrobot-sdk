package hikrobot

import (
	"os"
	"testing"
	"time"
)

// Hardware tests need the MVS runtime, so they run only when asked:
//
//	HIKROBOT_TEST=runtime  no camera needed: load, enumerate and convert
//	HIKROBOT_TEST=monitor  also reads camera nodes (GigE); works beside a program holding AccessControl
//	HIKROBOT_TEST=grab     exclusive: configures the camera and grabs frames
//
// HIKROBOT_SDK overrides the runtime path; HIKROBOT_CAMERA picks a camera by
// serial, IP, model or user name.
func loadTestSDK(t *testing.T) (*SDK, string) {
	mode := os.Getenv("HIKROBOT_TEST")
	if mode == "" {
		t.Skip("set HIKROBOT_TEST=runtime, monitor or grab to run against the MVS runtime")
	}
	sdk, err := Load(os.Getenv("HIKROBOT_SDK"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sdk.Close() })
	t.Logf("MVS runtime %s", sdk.Version())
	return sdk, mode
}

func openTestCamera(t *testing.T) (*SDK, *Camera, string) {
	sdk, mode := loadTestSDK(t)
	if mode == "runtime" {
		t.Skip("set HIKROBOT_TEST=monitor or grab to use a camera")
	}
	devices, err := sdk.Devices(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range devices {
		t.Logf("found %v (vendor %q, name %q, MAC %v, exclusive-accessible %v)",
			d, d.Vendor, d.UserName, d.MAC, sdk.Accessible(d, AccessExclusive))
	}
	if len(devices) == 0 {
		t.Fatal("no cameras found")
	}
	d := devices[0]
	if key := os.Getenv("HIKROBOT_CAMERA"); key != "" {
		if d, err = sdk.Find(key); err != nil {
			t.Fatal(err)
		}
	}
	access := AccessExclusive
	if mode == "monitor" {
		access = AccessMonitor
	}
	cam, err := sdk.OpenMode(d, access)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cam.Close(); err != nil {
			t.Error(err)
		}
	})
	return sdk, cam, mode
}

func TestHardwareNodes(t *testing.T) {
	_, cam, _ := openTestCamera(t)
	if !cam.Connected() {
		t.Error("camera reports disconnected")
	}
	width, err := cam.GetInt("Width")
	if err != nil || width.Current <= 0 {
		t.Fatalf("Width = %+v, %v", width, err)
	}
	format, err := cam.GetEnumString("PixelFormat")
	if err != nil || format == "" {
		t.Fatalf("PixelFormat = %q, %v", format, err)
	}
	enum, err := cam.GetEnum("PixelFormat")
	if err != nil || len(enum.Supported) == 0 {
		t.Fatalf("PixelFormat enum = %+v, %v", enum, err)
	}
	exposure, err := cam.GetFloat("ExposureTime")
	if err != nil {
		t.Fatal(err)
	}
	model, err := cam.GetString("DeviceModelName")
	if err != nil || model == "" {
		t.Fatalf("DeviceModelName = %q, %v", model, err)
	}
	t.Logf("%s: Width %+v, PixelFormat %s (%d supported), ExposureTime %+v", model, width, format, len(enum.Supported), exposure)
	if _, err := cam.GetInt("NoSuchNode"); err == nil {
		t.Error("read a node that does not exist")
	} else {
		t.Logf("missing node: %v", err)
	}
	if err := cam.OnException(func(e Exception) { t.Logf("exception: %v", e) }); err != nil {
		t.Error(err)
	}
}

// TestHardwareRuntime needs no camera: the SDK converts through a handle that
// was never opened, which also checks the pixel-conversion struct layout.
func TestHardwareRuntime(t *testing.T) {
	sdk, _ := loadTestSDK(t)
	if sdk.Version() == "" {
		t.Error("no runtime version")
	}
	devices, err := sdk.Devices(0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d camera(s)", len(devices))
	k, err := sdk.NewConverter()
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	const w, h = 64, 48
	data := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if y%2 == 1 && x%2 == 0 { // BayerGB8: odd rows start with red
				data[y*w+x] = 200
			}
		}
	}
	f, err := NewFrame(w, h, BayerGB8, data)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Release()
	img, err := k.Image(f)
	if err != nil {
		t.Fatal(err)
	}
	if c := img.RGBAAt(w/2, h/2); c.R < 100 || c.G > 60 || c.B > 60 {
		t.Fatalf("red-only Bayer frame converted to %+v", c)
	}
	small, err := k.Preview(f, 16)
	if err != nil || small.Bounds().Dx() != 16 || small.Bounds().Dy() != 12 {
		t.Fatalf("preview %v, %v", small.Bounds(), err)
	}
	bgr, err := k.ConvertTo(f, BGR8Packed, nil)
	if err != nil || len(bgr) != w*h*3 {
		t.Fatalf("BGR conversion: %d bytes, %v", len(bgr), err)
	}
	if mid := (h/2*w + w/2) * 3; bgr[mid+2] < 100 || bgr[mid] > 60 {
		t.Fatalf("BGR pixel = %v", bgr[mid:mid+3])
	}
	for _, q := range []BayerQuality{BayerFast, BayerOptimalPlus} {
		if err := k.SetBayerQuality(q); err != nil {
			t.Fatal(err)
		}
		if _, err := k.Image(f); err != nil {
			t.Fatalf("quality %d: %v", q, err)
		}
	}
}

func TestHardwareGrab(t *testing.T) {
	_, cam, mode := openTestCamera(t)
	if mode != "grab" {
		t.Skip("set HIKROBOT_TEST=grab to grab frames")
	}
	if err := cam.SetEnumString("TriggerMode", "Off"); err != nil {
		t.Fatal(err)
	}
	if err := cam.SetGrabStrategy(GrabLatestOnly); err != nil {
		t.Fatal(err)
	}
	if err := cam.StartGrabbing(); err != nil {
		t.Fatal(err)
	}
	var last uint32
	for i := 0; i < 5; i++ {
		f, err := cam.Grab(3 * time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("frame %d: %dx%d %v, %d bytes, lost %d, host %v", f.Number, f.Width, f.Height,
			f.PixelType, len(f.Data()), f.LostPackets, f.HostTime.Format(time.RFC3339Nano))
		if i > 0 && f.Number <= last {
			t.Errorf("frame number went from %d to %d", last, f.Number)
		}
		last = f.Number
		if i == 0 {
			img, err := cam.Preview(f, 640)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("preview %v", img.Bounds())
		}
		f.Release()
	}
	if err := cam.StopGrabbing(); err != nil {
		t.Fatal(err)
	}
	if _, err := cam.Grab(100 * time.Millisecond); err == nil {
		t.Error("grabbed a frame after StopGrabbing")
	}
}
