# go-hikrobot-sdk

[![Go Reference](https://pkg.go.dev/badge/github.com/bronystylecrazy/go-hikrobot-sdk.svg)](https://pkg.go.dev/github.com/bronystylecrazy/go-hikrobot-sdk)
[![CI](https://github.com/bronystylecrazy/go-hikrobot-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/bronystylecrazy/go-hikrobot-sdk/actions/workflows/ci.yml)

Go bindings for Hikrobot machine-vision cameras (GigE Vision, USB3 Vision and
GenTL frame grabbers) through the vendor's MVS runtime. **No cgo.**

It runs on **Windows, Linux and macOS**. The MVS runtime library is loaded
when the program runs, so:

- `go build` needs no C compiler, MVS headers or import libraries, and
  `CGO_ENABLED=0` works;
- you can cross-compile for any supported target from any machine;
- a machine without MVS still runs your program; `Load` just returns an
  error.

```go
import hikrobot "github.com/bronystylecrazy/go-hikrobot-sdk"
```

## Requirements

- Go 1.25 or newer.
- The [MVS client](https://www.hikrobotics.com/en/machinevision/service/download)
  for your OS, which installs the runtime library. `Load("")` looks here:

| Platform | Runtime `Load("")` opens |
|---|---|
| Windows amd64 | `C:\Program Files (x86)\Common Files\MVS\Runtime\Win64_x64\MvCameraControl.dll` |
| Linux amd64 | `$MVCAM_COMMON_RUNENV/64/libMvCameraControl.so`, else `/opt/MVS/lib/64/…`, else the loader's search path |
| Linux arm64 | `$MVCAM_COMMON_RUNENV/aarch64/libMvCameraControl.so`, else `/opt/MVS/lib/aarch64/…`, else the loader's search path |
| macOS amd64 / arm64 | `libMvCameraControl.dylib` on the loader's search path |

Pass a path to `Load` to use another location. Other platforms compile, but
`Load` returns `ErrUnsupported`. MVS 4.1 and newer runtimes are supported;
struct layouts are checked against MVS 4.8 (Windows) and 5.1 (Linux).

**Linux:** the runtime loads its sibling libraries (pixel conversion,
transport layers) by name, so its directory must be on the loader's search
path *when your program starts*; a running process cannot add it. Either
start the program with the MVS environment, as the installer's
`set_env_path.sh` sets up for login shells:

```sh
export MVCAM_COMMON_RUNENV=/opt/MVS/lib
export LD_LIBRARY_PATH=/opt/MVS/lib/aarch64:$LD_LIBRARY_PATH   # /opt/MVS/lib/64 on x86_64
```

or register it once, system-wide, which also covers systemd services:

```sh
echo /opt/MVS/lib/aarch64 | sudo tee /etc/ld.so.conf.d/mvs.conf && sudo ldconfig
```

Without either, `Load` and enumeration still work, but conversion fails with
"failed to load library".

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
| Conversion | `Image` (RGBA), `Preview` (scaled RGBA), `ConvertTo` (any SDK output, e.g. `BGR8Packed` for OpenCV), `SetBayerQuality`; on a `Camera`, or on a camera-free `SDK.NewConverter` for saved raw frames |

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

`go test ./...` runs everywhere. Besides unit tests for struct layouts, device
parsing, frame ownership and Bayer subsampling, it builds
[`testdata/fakemvs`](testdata/fakemvs/fakemvs.c), a small stand-in for the MVS
runtime written in C, and drives every binding through the real loading and
calling path: integer, float and bool arguments, struct pointers, frame
buffers, and exception callbacks arriving on a foreign thread. CI runs this on
Linux, macOS and Windows, with and without cgo. It needs a C compiler (`cc`,
or `gcc` on Windows) and is skipped without one.

With MVS installed, the hardware tests run when asked:

```sh
HIKROBOT_TEST=runtime go test -v -run Hardware   # no camera: load, enumerate, convert with the real SDK
HIKROBOT_TEST=monitor go test -v -run Hardware   # + read camera nodes beside a program holding AccessControl
HIKROBOT_TEST=grab go test -v -run Hardware      # exclusive: configures the camera and grabs
# optional: HIKROBOT_CAMERA=<serial, IP, model or name>, HIKROBOT_SDK=<runtime path>
```

`runtime` mode has passed against MVS 4.8 on Windows x64 and MVS 5.1 on Linux
aarch64.

## How it works

Each SDK function is resolved from the runtime on first use. Windows calls
go through `syscall`; Linux and macOS use
[purego](https://github.com/ebitengine/purego) (`dlopen` without cgo), with
the one float-taking function bound by signature so the value lands in a
vector register. Go mirrors of the C structs live in [`layout.go`](layout.go), and
`layout_test.go` pins every size and offset to values compiled from
`CameraParams.h`. One difference matters: `MvGvspPixelType` holds both `-1`
and values above `0x7FFFFFFF`, so GCC and Clang make it 8 bytes while MSVC
makes it 4, and the frame-info and conversion structs differ between Windows
and Linux/macOS. The Linux layout was confirmed against the real runtime. Functions missing from an older runtime fail only when you
call them. No Hikrobot code or headers are included in this repository.

## License

MIT. The MVS runtime is Hikrobot's software, distributed under its own
licence; install it from Hikrobot.
