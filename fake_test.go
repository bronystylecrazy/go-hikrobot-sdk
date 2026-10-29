//go:build (windows && amd64) || ((linux || darwin) && (amd64 || arm64))

package hikrobot

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// loadFake compiles testdata/fakemvs into a shared library and loads it, so
// the tests below run every binding through the real call path. They skip
// when no C compiler is available.
func loadFake(t *testing.T) *SDK {
	t.Helper()
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
		if runtime.GOOS == "windows" {
			cc = "gcc"
		}
	}
	if _, err := exec.LookPath(cc); err != nil {
		t.Skipf("no C compiler (%s) to build the fake MVS runtime", cc)
	}
	name := map[string]string{"windows": "fakemvs.dll", "darwin": "libfakemvs.dylib"}[runtime.GOOS]
	if name == "" {
		name = "libfakemvs.so"
	}
	lib := filepath.Join(t.TempDir(), name)
	args := []string{"-shared", "-O1", "-o", lib, filepath.Join("testdata", "fakemvs", "fakemvs.c")}
	if runtime.GOOS != "windows" {
		args = append(args, "-fPIC", "-pthread")
	}
	if out, err := exec.Command(cc, args...).CombinedOutput(); err != nil {
		t.Fatalf("build fake runtime: %v\n%s", err, out)
	}
	sdk, err := Load(lib)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sdk.Close() })
	return sdk
}

func openFake(t *testing.T) (*SDK, *Camera) {
	t.Helper()
	sdk := loadFake(t)
	d, err := sdk.Find("FAKE0001")
	if err != nil {
		t.Fatal(err)
	}
	cam, err := sdk.Open(d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cam.Close() })
	return sdk, cam
}

func TestFakeDiscovery(t *testing.T) {
	sdk := loadFake(t)
	if v := sdk.Version(); v != "4.8.2.2" {
		t.Fatalf("Version = %q", v)
	}
	devices, err := sdk.Devices(0)
	if err != nil || len(devices) != 1 {
		t.Fatalf("Devices = %v, %v", devices, err)
	}
	d := devices[0]
	if d.String() != "GigE FAKE-GIGE #FAKE0001 @ 192.168.1.50" || d.Vendor != "FakeVendor" ||
		d.UserName != "bench" || d.MAC.String() != "00:11:22:33:44:55" {
		t.Fatalf("device %v %+v", d, d)
	}
	if none, err := sdk.Devices(TransportUSB3); err != nil || len(none) != 0 {
		t.Fatalf("USB3 devices = %v, %v", none, err)
	}
	for _, key := range []string{"fake0001", "192.168.1.50", "FAKE-GIGE", "bench"} {
		if _, err := sdk.Find(key); err != nil {
			t.Errorf("Find(%q): %v", key, err)
		}
	}
	if _, err := sdk.Find("nope"); err == nil {
		t.Error("found a camera that does not exist")
	}

	if !sdk.Accessible(d, AccessExclusive) {
		t.Fatal("free camera reported busy")
	}
	cam, err := sdk.Open(d)
	if err != nil {
		t.Fatal(err)
	}
	if sdk.Accessible(d, AccessExclusive) {
		t.Error("open camera reported free")
	}
	if _, err := sdk.Open(d); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("second open: %v", err)
	}
	if !cam.Connected() || cam.Device().Serial != "FAKE0001" {
		t.Error("camera not connected")
	}
	if size, _ := cam.GetInt("GevSCPSPacketSize"); size.Current != 8164 {
		t.Errorf("Open did not apply the optimal packet size: %+v", size)
	}
	if err := cam.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cam.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if cam.Connected() {
		t.Error("closed camera reports connected")
	}
	if _, err := cam.GetInt("Width"); !errors.Is(err, ErrClosed) {
		t.Errorf("call after Close: %v", err)
	}
}

func TestFakeNodes(t *testing.T) {
	_, cam := openFake(t)

	if err := cam.SetInt("Width", 32); err != nil {
		t.Fatal(err)
	}
	if v, err := cam.GetInt("Width"); err != nil || v.Current != 32 || v.Increment != 8 {
		t.Fatalf("Width = %+v, %v", v, err)
	}
	if _, err := cam.GetInt("Missing"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("missing node: %v", err)
	} else if want := "hikrobot: MV_CC_GetIntValueEx(Missing): GenICam: node not found (0x80000109)"; err.Error() != want {
		t.Fatalf("error text %q", err)
	}

	// Floats travel in vector registers; a wrong convention garbles them.
	for _, v := range []float32{20000.5, 15, 999999} {
		if err := cam.SetFloat("ExposureTime", v); err != nil {
			t.Fatal(err)
		}
		if got, err := cam.GetFloat("ExposureTime"); err != nil || got.Current != v || got.Min != 15 {
			t.Fatalf("ExposureTime = %+v, %v; want %v", got, err, v)
		}
	}
	if err := cam.SetFloat("ExposureTime", 1); err == nil {
		t.Fatal("accepted an out-of-range exposure")
	}

	if err := cam.SetEnumString("PixelFormat", "Mono8"); err != nil {
		t.Fatal(err)
	}
	if s, err := cam.GetEnumString("PixelFormat"); err != nil || s != "Mono8" {
		t.Fatalf("PixelFormat = %q, %v", s, err)
	}
	if err := cam.SetEnum("PixelFormat", uint32(BayerGB8)); err != nil {
		t.Fatal(err)
	}
	e, err := cam.GetEnum("PixelFormat")
	if err != nil || PixelType(e.Current) != BayerGB8 || len(e.Supported) != 2 {
		t.Fatalf("PixelFormat enum = %+v, %v", e, err)
	}
	if s, err := cam.EnumSymbol("TriggerSource", 7); err != nil || s != "Software" {
		t.Fatalf("EnumSymbol = %q, %v", s, err)
	}

	for _, v := range []bool{true, false, true} {
		if err := cam.SetBool("ReverseX", v); err != nil {
			t.Fatal(err)
		}
		if got, err := cam.GetBool("ReverseX"); err != nil || got != v {
			t.Fatalf("ReverseX = %v, %v; want %v", got, err, v)
		}
	}

	if err := cam.SetString("DeviceUserID", "line-3"); err != nil {
		t.Fatal(err)
	}
	if s, err := cam.GetString("DeviceUserID"); err != nil || s != "line-3" {
		t.Fatalf("DeviceUserID = %q, %v", s, err)
	}
	if err := cam.SetString("DeviceUserID", "bad\x00name"); err == nil {
		t.Fatal("accepted a NUL in a string")
	}

	if err := cam.Execute("Missing"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("Execute(Missing): %v", err)
	}

	path := filepath.Join(t.TempDir(), "camera.mfs")
	if err := cam.SetFloat("ExposureTime", 1234); err != nil {
		t.Fatal(err)
	}
	if err := cam.SaveFeatures(path); err != nil {
		t.Fatal(err)
	}
	_ = cam.SetFloat("ExposureTime", 99)
	if err := cam.LoadFeatures(path); err != nil {
		t.Fatal(err)
	}
	if v, _ := cam.GetFloat("ExposureTime"); v.Current != 1234 {
		t.Fatalf("LoadFeatures restored %v", v.Current)
	}

	if err := cam.SetResend(true, 10, 75*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if v, _ := cam.GetInt("FakeResendTimeout"); v.Current != 75 {
		t.Fatalf("resend timeout %d", v.Current)
	}
	for name, err := range map[string]error{
		"SetGrabStrategy":    cam.SetGrabStrategy(GrabLatestOnly),
		"SetImageNodeNum":    cam.SetImageNodeNum(4),
		"SetOutputQueueSize": cam.SetOutputQueueSize(2),
		"ClearImageBuffer":   cam.ClearImageBuffer(),
		"SetBayerQuality":    cam.SetBayerQuality(BayerOptimalPlus),
	} {
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := cam.SetGrabStrategy(GrabStrategy(9)); !errors.Is(err, ErrParameter) {
		t.Errorf("invalid strategy: %v", err)
	}
}

func TestFakeGrab(t *testing.T) {
	_, cam := openFake(t)
	if _, err := cam.Grab(time.Millisecond); err == nil {
		t.Fatal("grabbed before StartGrabbing")
	}
	if err := cam.StartGrabbing(); err != nil {
		t.Fatal(err)
	}
	if err := cam.SetInt("Width", 16); !errors.Is(err, ErrNodeAccess) {
		t.Fatalf("Width writable while grabbing: %v", err)
	}
	var frames []*Frame
	for i := 1; i <= 3; i++ {
		f, err := cam.Grab(time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if f.Number != uint32(i) || f.Width != 64 || f.Height != 48 || f.PixelType != BayerGB8 ||
			len(f.Data()) != 64*48 || f.LostPackets != uint32(i%2) || f.DeviceTimestamp != 1<<32|uint64(i) ||
			!f.HostTime.Equal(time.UnixMilli(1700000000000+int64(i))) {
			t.Fatalf("frame %d: %+v", i, f)
		}
		if got := f.Data()[10*64+5]; got != byte(i+5+10) {
			t.Fatalf("frame %d pixel = %d", i, got)
		}
		frames = append(frames, f)
	}
	if n, _ := cam.GetInt("FakeOutstanding"); n.Current != 0 {
		t.Fatalf("%d SDK buffers were not freed", n.Current)
	}
	// Each frame owns its pixels even though the buffers came from one pool.
	if frames[0].Data()[0] != 1 || frames[2].Data()[0] != 3 {
		t.Fatal("frames share storage")
	}
	for _, f := range frames {
		f.Release()
	}

	f, err := cam.Grab(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	img, err := cam.Image(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 48 {
		t.Fatalf("image %v", b)
	}
	v := f.Data()[3*64+7]
	if c := img.RGBAAt(7, 3); c.R != v || c.G != v/2 || c.B != 255-v || c.A != 255 {
		t.Fatalf("pixel %+v from %d", c, v)
	}
	small, err := cam.Preview(f, 16)
	if err != nil || small.Bounds().Dx() != 16 || small.Bounds().Dy() != 12 {
		t.Fatalf("preview %v, %v", small.Bounds(), err)
	}
	bgr, err := cam.ConvertTo(f, BGR8Packed, nil)
	if err != nil || len(bgr) != 64*48*3 || bgr[0] != 255-f.Data()[0] || bgr[2] != f.Data()[0] {
		t.Fatalf("BGR: %d bytes, %v", len(bgr), err)
	}
	if _, err := cam.ConvertTo(f, YUV422, nil); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("YUV conversion: %v", err)
	}
	f.Release()
	if _, err := cam.Image(f); err == nil {
		t.Fatal("converted a released frame")
	}

	if err := cam.StopGrabbing(); err != nil {
		t.Fatal(err)
	}
}

func TestFakeSoftwareTrigger(t *testing.T) {
	_, cam := openFake(t)
	if err := cam.SetEnumString("TriggerMode", "On"); err != nil {
		t.Fatal(err)
	}
	if err := cam.SetEnumString("TriggerSource", "Software"); err != nil {
		t.Fatal(err)
	}
	if err := cam.StartGrabbing(); err != nil {
		t.Fatal(err)
	}
	if _, err := cam.Grab(10 * time.Millisecond); !errors.Is(err, ErrTimeout) {
		t.Fatalf("grab without trigger: %v", err)
	}
	if err := cam.Execute("TriggerSoftware"); err != nil {
		t.Fatal(err)
	}
	f, err := cam.Grab(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	f.Release()
}

func TestFakeExceptionCallback(t *testing.T) {
	_, cam := openFake(t)
	got := make(chan Exception, 2)
	if err := cam.OnException(func(e Exception) { got <- e }); err != nil {
		t.Fatal(err)
	}
	// The fake calls back from a thread the Go runtime has never seen.
	if err := cam.Execute("FakeDisconnect"); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-got:
		if e != ExceptionDisconnected {
			t.Fatalf("got %v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no exception callback")
	}

	replaced := make(chan Exception, 1)
	if err := cam.OnException(func(e Exception) { replaced <- e }); err != nil {
		t.Fatal(err)
	}
	if err := cam.Execute("FakeDisconnect"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-replaced:
	case <-got:
		t.Fatal("old handler still called")
	case <-time.After(5 * time.Second):
		t.Fatal("no exception callback after replacement")
	}
}

// Close must wait for a Grab in progress rather than free the handle under it.
func TestFakeCloseWhileGrabbing(t *testing.T) {
	_, cam := openFake(t)
	if err := cam.StartGrabbing(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			f, err := cam.Grab(time.Second)
			if err != nil {
				if !errors.Is(err, ErrClosed) {
					t.Errorf("grab: %v", err)
				}
				return
			}
			f.Release()
		}
	}()
	time.Sleep(20 * time.Millisecond)
	if err := cam.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
}
