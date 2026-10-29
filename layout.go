package hikrobot

// Go mirrors of the MVS C structs, checked against CameraParams.h from MVS
// 4.8 compiled for x86_64-pc-windows-msvc (see layout_test.go). Pointers and
// int64 fields are 8-byte aligned, as in Go on 64-bit targets.

// MV_CC_DEVICE_INFO. Special is the per-transport union.
type deviceInfo struct {
	Major, Minor                           uint16
	MACHigh, MACLow, Transport, DeviceType uint32
	Reserved                               [3]uint32
	Special                                [604]byte
}

// MV_CC_DEVICE_INFO_LIST
type deviceList struct {
	Count   uint32
	Devices [256]*deviceInfo
}

// MVCC_INTVALUE_EX
type intValue struct {
	Current, Max, Min, Increment int64
	Reserved                     [16]uint32
}

// MVCC_FLOATVALUE
type floatValue struct {
	Current, Max, Min float32
	Reserved          [4]uint32
}

// MVCC_ENUMVALUE
type enumValue struct {
	Current, Count uint32
	Supported      [64]uint32
	Reserved       [4]uint32
}

// MVCC_ENUMENTRY
type enumEntry struct {
	Value    uint32
	Symbol   [64]byte
	Reserved [4]uint32
}

// MVCC_STRINGVALUE
type stringValue struct {
	Current   [256]byte
	MaxLength int64
	Reserved  [2]uint32
}

// MV_FRAME_OUT_INFO_EX
type frameInfo struct {
	Width, Height                                                 uint16
	PixelType, Number, DeviceTimeHigh, DeviceTimeLow, Reserved0   uint32
	HostTime                                                      int64
	Length, Seconds, Cycles, CycleOffset                          uint32
	Gain, Exposure                                                float32
	Brightness, Red, Green, Blue, Counter, Trigger, Input, Output uint32
	OffsetX, OffsetY, ChunkWidth, ChunkHeight                     uint16
	LostPackets, ChunkCount                                       uint32
	ChunkList                                                     uint64
	ExtendedWidth, ExtendedHeight                                 uint32
	ExtendedLength                                                uint64
	ExtraType, SubImageCount                                      uint32
	SubImages, User                                               uint64
	FirstEncoder, LastEncoder, LastFrame                          uint32
	Reserved                                                      [23]uint32
}

// MV_FRAME_OUT
type frameOut struct {
	Data     *byte
	Info     frameInfo
	Reserved [16]uint32
}

// MV_CC_PIXEL_CONVERT_PARAM_EX
type pixelConvert struct {
	Width, Height, SourcePixelType     uint32
	Source                             *byte
	SourceLength, DestinationPixelType uint32
	Destination                        *byte
	DestinationLength, DestinationSize uint32
	Reserved                           [4]uint32
}
