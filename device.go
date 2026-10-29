package hikrobot

import (
	"bytes"
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// Transport is a transport-layer bit (MV_*_DEVICE). Combine them to choose
// what SDK.Devices enumerates.
type Transport uint32

const (
	TransportGigE           Transport = 0x00000001
	TransportUSB3           Transport = 0x00000004
	TransportCameraLink     Transport = 0x00000008
	TransportGenTLGigE      Transport = 0x00000040
	TransportGenTLCameraLnk Transport = 0x00000080
	TransportGenTLCXP       Transport = 0x00000100
	TransportGenTLXoF       Transport = 0x00000200
	TransportGenTLVirtual   Transport = 0x00000800
	TransportGenTLXoC       Transport = 0x00001000
)

var transportNames = []struct {
	t    Transport
	name string
}{
	{TransportGigE, "GigE"}, {TransportUSB3, "USB3"}, {TransportCameraLink, "CameraLink"},
	{TransportGenTLGigE, "GenTL-GigE"}, {TransportGenTLCameraLnk, "GenTL-CameraLink"},
	{TransportGenTLCXP, "GenTL-CXP"}, {TransportGenTLXoF, "GenTL-XoF"},
	{TransportGenTLVirtual, "GenTL-Virtual"}, {TransportGenTLXoC, "GenTL-XoC"},
}

func (t Transport) String() string {
	var names []string
	for _, n := range transportNames {
		if t&n.t != 0 {
			names = append(names, n.name)
			t &^= n.t
		}
	}
	if t != 0 || len(names) == 0 {
		names = append(names, fmt.Sprintf("0x%X", uint32(t)))
	}
	return strings.Join(names, "|")
}

// Device describes one enumerated camera. Pass it to SDK.Open; a Device built
// by hand cannot be opened.
type Device struct {
	Transport Transport
	Vendor    string
	Model     string
	Serial    string
	// UserName is the camera's user-defined name (DeviceUserID).
	UserName string
	// IP is the current address of a GigE camera; invalid otherwise.
	IP netip.Addr
	// MAC is set for GigE cameras.
	MAC net.HardwareAddr

	info deviceInfo
}

func (d Device) String() string {
	s := d.Transport.String() + " " + d.Model
	if d.Serial != "" {
		s += " #" + d.Serial
	}
	if d.IP.IsValid() {
		s += " @ " + d.IP.String()
	}
	return s
}

func cstr(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}

// parseDevice decodes the SpecialInfo union. Offsets are from CameraParams.h;
// the GenTL frame-grabber variants share one layout.
func parseDevice(info *deviceInfo) Device {
	d := Device{Transport: Transport(info.Transport), info: *info}
	b := info.Special[:]
	switch d.Transport {
	case TransportGigE, TransportGenTLGigE:
		d.IP = netip.AddrFrom4([4]byte{b[11], b[10], b[9], b[8]})
		d.Vendor, d.Model = cstr(b[20:52]), cstr(b[52:84])
		d.Serial, d.UserName = cstr(b[164:180]), cstr(b[180:196])
		d.MAC = net.HardwareAddr{
			byte(info.MACHigh >> 8), byte(info.MACHigh),
			byte(info.MACLow >> 24), byte(info.MACLow >> 16), byte(info.MACLow >> 8), byte(info.MACLow),
		}
	case TransportUSB3:
		d.Vendor, d.Model = cstr(b[76:140]), cstr(b[140:204])
		d.Serial, d.UserName = cstr(b[396:460]), cstr(b[460:524])
	case TransportCameraLink:
		d.Model, d.Vendor, d.Serial = cstr(b[64:128]), cstr(b[256:320]), cstr(b[320:384])
	case TransportGenTLCameraLnk, TransportGenTLCXP, TransportGenTLXoF, TransportGenTLVirtual, TransportGenTLXoC:
		d.Vendor, d.Model = cstr(b[64:128]), cstr(b[128:192])
		d.Serial, d.UserName = cstr(b[320:384]), cstr(b[384:448])
	}
	return d
}

// AccessMode is how SDK.OpenMode claims the camera (MV_ACCESS_*).
type AccessMode uint32

const (
	// AccessExclusive gives full control; other programs may only read the
	// control channel register.
	AccessExclusive AccessMode = 1
	// AccessControl gives full control while letting other programs read
	// every register.
	AccessControl AccessMode = 3
	// AccessMonitor opens read-only alongside a program that opened the
	// camera with AccessControl (an AccessExclusive holder refuses it).
	// GigE only.
	AccessMonitor AccessMode = 7
)

// GrabStrategy is how Grab picks frames from the SDK's queue
// (MV_GRAB_STRATEGY).
type GrabStrategy uint32

const (
	// GrabOneByOne returns every frame in arrival order (SDK default).
	GrabOneByOne GrabStrategy = 0
	// GrabLatestOnly drops everything but the newest frame. Use it for live
	// views where a slow consumer must not fall behind.
	GrabLatestOnly GrabStrategy = 1
	// GrabLatest keeps the newest frames, up to SetOutputQueueSize of them.
	GrabLatest GrabStrategy = 2
	// GrabUpcoming ignores queued frames and waits for the next one.
	GrabUpcoming GrabStrategy = 3
)

// BayerQuality is the SDK's demosaicing method for Bayer conversions.
type BayerQuality uint32

const (
	BayerFast        BayerQuality = 0
	BayerBalanced    BayerQuality = 1
	BayerOptimal     BayerQuality = 2
	BayerOptimalPlus BayerQuality = 3
)

// Exception is an asynchronous device event delivered to Camera.OnException.
type Exception uint32

const (
	ExceptionDisconnected Exception = 0x00008001
	ExceptionVersionCheck Exception = 0x00008002
)

func (e Exception) String() string {
	switch e {
	case ExceptionDisconnected:
		return "device disconnected"
	case ExceptionVersionCheck:
		return "SDK and driver versions do not match"
	}
	return fmt.Sprintf("exception 0x%X", uint32(e))
}
