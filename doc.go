// Package hikrobot drives Hikrobot machine-vision cameras (GigE Vision, USB3
// Vision and GenTL frame grabbers) through the vendor's MVS runtime,
// MvCameraControl.dll, without cgo.
//
// The runtime is loaded by Load at run time, so programs build on any platform
// and cross-compile without the MVS headers or a C toolchain. Capture works on
// windows/amd64 with MVS installed; elsewhere Load returns ErrUnsupported.
//
// A minimal capture loop:
//
//	sdk, err := hikrobot.Load("") // default MVS install path
//	if err != nil { ... }
//	defer sdk.Close()
//	devices, err := sdk.Devices(0)
//	if err != nil || len(devices) == 0 { ... }
//	cam, err := sdk.Open(devices[0])
//	if err != nil { ... }
//	defer cam.Close()
//	_ = cam.SetEnumString("TriggerMode", "Off")
//	if err := cam.StartGrabbing(); err != nil { ... }
//	frame, err := cam.Grab(time.Second)
//	if err != nil { ... }
//	defer frame.Release()
//	img, err := cam.Image(frame) // *image.RGBA, debayered by the SDK
//
// Frames are copied out of the SDK's buffer before Grab returns, so the SDK
// buffer is always given back. The copies come from a per-camera pool: call
// Release once you are done, and Retain before sharing a frame with another
// goroutine.
package hikrobot
