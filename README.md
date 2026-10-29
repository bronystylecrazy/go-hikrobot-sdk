# go-hikrobot-sdk

[![Go Reference](https://pkg.go.dev/badge/github.com/bronystylecrazy/go-hikrobot-sdk.svg)](https://pkg.go.dev/github.com/bronystylecrazy/go-hikrobot-sdk)
[![CI](https://github.com/bronystylecrazy/go-hikrobot-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/bronystylecrazy/go-hikrobot-sdk/actions/workflows/ci.yml)

Go bindings for Hikrobot machine-vision cameras (GigE Vision, USB3 Vision and
GenTL frame grabbers) through the vendor's MVS runtime. **No cgo.**

The library loads `MvCameraControl.dll` at run time, so:

- `go build` needs no C compiler, MVS headers or import libraries;
- you can cross-compile a Windows camera program from macOS or Linux;
- a machine without MVS still runs your program; `Load` just returns an
  error.

```go
import hikrobot "github.com/bronystylecrazy/go-hikrobot-sdk"
```

## Requirements

- **Windows amd64** with the [MVS client or runtime](https://www.hikrobotics.com/en/machinevision/service/download)
  installed. The library looks for
  `C:\Program Files (x86)\Common Files\MVS\Runtime\Win64_x64\MvCameraControl.dll`;
  pass another path to `Load` if yours differs.
- Go 1.22 or newer.

Other platforms compile, but `Load` returns `ErrUnsupported` there. MVS 4.1 and
newer runtimes are supported; struct layouts are checked against MVS 4.8.

## Quick start

```go
sdk, err := hikrobot.Load("") // "" = default MVS install path
if err != nil {
	log.Fatal(err)
}
defer sdk.Close()

devices, err := sdk.Devices(0) // 0 = GigE + USB3
if err != nil || len(devices) == 0 {
	log.Fatal("no camera", err)
}
cam, err := sdk.Open(devices[0])
if err != nil {
	log.Fatal(err) // errors.Is(err, hikrobot.ErrAccessDenied) if another program holds it
}
defer cam.Close()

cam.SetEnumString("TriggerMode", "Off")
cam.SetFloat("ExposureTime", 20000) // µs
cam.SetGrabStrategy(hikrobot.GrabLatestOnly)
if err := cam.StartGrabbing(); err != nil {
	log.Fatal(err)
}

frame, err := cam.Grab(time.Second)
if err != nil {
	log.Fatal(err) // errors.Is(err, hikrobot.ErrTimeout) when no frame arrived
}
defer frame.Release()

img, err := cam.Image(frame) // *image.RGBA, debayered by the SDK
```

Runnable programs are in [`examples/`](examples): `list` prints the cameras
and whether each is free; `grab` captures frames (free-running or
software-triggered) and saves a PNG.

## What's covered

| Area | API |
|---|---|
| Runtime | `Load`, `SDK.Close`, `SDK.Version` |
| Discovery | `SDK.Devices`, `SDK.Find` (by serial, IP, model or name), `SDK.Accessible` |
| Connection | `SDK.Open`, `SDK.OpenMode` (exclusive / control / monitor), `Camera.Connected`, `Camera.OnException` (disconnect events) |
| GenICam nodes | `GetInt`/`SetInt`, `GetFloat`/`SetFloat`, `GetEnum`/`SetEnum`/`SetEnumString`/`GetEnumString`/`EnumSymbol`, `GetBool`/`SetBool`, `GetString`/`SetString`, `Execute` (commands such as `TriggerSoftware`) |
| Settings files | `SaveFeatures`, `LoadFeatures` |
| Acquisition | `StartGrabbing`, `StopGrabbing`, `Grab`, `SetGrabStrategy`, `SetOutputQueueSize`, `SetImageNodeNum`, `ClearImageBuffer` |
| GigE transport | `OptimalPacketSize` (applied on open), `SetResend` |
| Conversion | `Image` (RGBA), `Preview` (scaled RGBA), `ConvertTo` (any SDK output, e.g. `BGR8Packed` for OpenCV), `SetBayerQuality` |

Anything else in the GenICam feature tree is reachable through the node
accessors by name; see the camera's feature list in the MVS client.

## Frames

`Grab` copies each image out of the SDK's buffer and gives the buffer back
before returning, so a slow consumer never starves the SDK. The copies come
from a per-camera pool and are reference-counted:

- you own the frame `Grab` returns; call `Release` once;
- call `Retain` before handing a frame to another goroutine, and have that
  goroutine `Release` it;
- `Data()` is valid only while you hold a reference, and must not be modified.

`Preview(frame, width)` is built for live views of large sensors. For Mono8
and 8-bit Bayer frames it subsamples whole 2×2 colour cells *before*
demosaicing, so a 1620-pixel preview of a 20 MP BayerGB8 frame costs a
fraction of a full conversion (about 1 ms for the subsampling).

## Errors

SDK failures are `*hikrobot.Error` values carrying the SDK function and its
`MV_E_*` code, with a readable description. Match them with `errors.Is`:

```go
if errors.Is(err, hikrobot.ErrTimeout) { /* no frame yet */ }
if errors.Is(err, hikrobot.ErrAccessDenied) { /* camera open elsewhere */ }
if errors.Is(err, hikrobot.ErrNodeNotFound) { /* feature not on this model */ }
```

## Testing

`go test ./...` runs everywhere. It covers struct layouts against the MVS
headers, device parsing, frame ownership and Bayer subsampling.

On Windows with a camera attached, the hardware tests run when asked:

```powershell
$env:HIKROBOT_TEST = "grab"      # exclusive: configures the camera and grabs
# or "monitor": read-only, beside a program that opened with AccessControl
$env:HIKROBOT_CAMERA = "DA1234567" # optional: serial, IP, model or name
go test -v -run Hardware
```

## How it works

Each SDK function is resolved from the DLL on first use and called through
`syscall`. Go mirrors of the C structs live in [`layout.go`](layout.go), and
`layout_test.go` pins every size and offset to values compiled from
`CameraParams.h`. Functions missing from an older runtime fail only when you
call them. No Hikrobot code or headers are included in this repository.

## License

MIT. The MVS runtime is Hikrobot's software, distributed under its own
licence; install it from Hikrobot.
