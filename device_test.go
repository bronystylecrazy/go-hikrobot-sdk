package hikrobot

import (
	"net/netip"
	"testing"
)

func TestParseGigEDevice(t *testing.T) {
	info := deviceInfo{Transport: uint32(TransportGigE), MACHigh: 0x0011, MACLow: 0x22334455}
	b := info.Special[:]
	b[8], b[9], b[10], b[11] = 50, 1, 168, 192 // nCurrentIp, little-endian
	copy(b[20:], "Hikrobot\x00")
	copy(b[52:], "MV-CS200-10GC\x00")
	copy(b[164:], "DA1234567\x00")
	copy(b[180:], "station-1\x00")
	d := parseDevice(&info)
	if d.Transport != TransportGigE || d.Vendor != "Hikrobot" || d.Model != "MV-CS200-10GC" ||
		d.Serial != "DA1234567" || d.UserName != "station-1" {
		t.Fatalf("parsed %+v", d)
	}
	if d.IP != netip.MustParseAddr("192.168.1.50") {
		t.Fatalf("IP = %v", d.IP)
	}
	if d.MAC.String() != "00:11:22:33:44:55" {
		t.Fatalf("MAC = %v", d.MAC)
	}
	if got := d.String(); got != "GigE MV-CS200-10GC #DA1234567 @ 192.168.1.50" {
		t.Fatalf("String() = %q", got)
	}
}

func TestParseUSB3Device(t *testing.T) {
	info := deviceInfo{Transport: uint32(TransportUSB3)}
	b := info.Special[:]
	copy(b[76:], "Hikrobot")
	copy(b[140:], "MV-CA050-20UC")
	copy(b[396:], "00D12345")
	d := parseDevice(&info)
	if d.Model != "MV-CA050-20UC" || d.Serial != "00D12345" || d.Vendor != "Hikrobot" || d.IP.IsValid() {
		t.Fatalf("parsed %+v", d)
	}
}

func TestTransportString(t *testing.T) {
	if got := (TransportGigE | TransportUSB3).String(); got != "GigE|USB3" {
		t.Fatal(got)
	}
	if got := Transport(0x4000).String(); got != "0x4000" {
		t.Fatal(got)
	}
}

func TestHandMadeDeviceCannotOpen(t *testing.T) {
	var s SDK
	if _, err := s.Open(Device{Model: "x"}); err == nil {
		t.Fatal("opened a device that was not enumerated")
	}
	if s.Accessible(Device{}, AccessExclusive) {
		t.Fatal("hand-made device reported accessible")
	}
}
