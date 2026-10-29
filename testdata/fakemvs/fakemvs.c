// fakemvs is a stand-in for the MVS runtime used by fake_test.go. It exports
// the subset of the MvCameraControl C API that this package binds, models one
// GigE camera, and is written from the public API description only. It lets
// CI exercise the real call path (argument passing, floats, bools, struct
// pointers and callbacks from foreign threads) on every OS without a camera.

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <windows.h>
#define API __declspec(dllexport)
#else
#include <pthread.h>
#define API __attribute__((visibility("default")))
#endif

// C enum MvGvspPixelType: MSVC makes it 4 bytes; GCC and Clang make it 8
// because it holds both -1 and values above 0x7FFFFFFF.
#ifdef _WIN32
typedef uint32_t pixel_enum;
#else
typedef uint64_t pixel_enum;
#endif

typedef struct {
	uint16_t major, minor;
	uint32_t mac_high, mac_low, transport, dev_type, reserved[3];
	unsigned char special[604];
} device_info;
typedef struct {
	uint32_t count;
	device_info *devices[256];
} device_list;
typedef struct {
	int64_t cur, max, min, inc;
	uint32_t reserved[16];
} int_value;
typedef struct {
	float cur, max, min;
	uint32_t reserved[4];
} float_value;
typedef struct {
	uint32_t cur, count, supported[64], reserved[4];
} enum_value;
typedef struct {
	uint32_t value;
	char symbol[64];
	uint32_t reserved[4];
} enum_entry;
typedef struct {
	char cur[256];
	int64_t max_length;
	uint32_t reserved[2];
} string_value;
typedef struct {
	uint16_t width, height;
	pixel_enum pixel_type;
	uint32_t number, dev_time_high, dev_time_low, reserved0;
	int64_t host_time;
	uint32_t length, seconds, cycles, cycle_offset;
	float gain, exposure;
	uint32_t brightness, red, green, blue, counter, trigger, input, output;
	uint16_t offset_x, offset_y, chunk_width, chunk_height;
	uint32_t lost_packets, chunk_count;
	uint64_t chunk_list;
	uint32_t ext_width, ext_height;
	uint64_t ext_length;
	uint32_t extra_type, sub_image_count;
	uint64_t sub_images, user;
	uint32_t first_encoder, last_encoder, last_frame, reserved[23];
} frame_info;
typedef struct {
	unsigned char *data;
	frame_info info;
	uint32_t reserved[16];
} frame_out;
typedef struct {
	uint32_t width, height;
	pixel_enum src_type;
	unsigned char *src;
	uint32_t src_len;
	pixel_enum dst_type;
	unsigned char *dst;
	uint32_t dst_len, dst_size, reserved[4];
} convert_param;

_Static_assert(sizeof(device_info) == 636, "device_info");
#ifdef _WIN32
_Static_assert(sizeof(frame_info) == 256, "frame_info");
_Static_assert(sizeof(frame_out) == 328, "frame_out");
_Static_assert(sizeof(convert_param) == 64, "convert_param");
#else
_Static_assert(sizeof(frame_info) == 264, "frame_info");
_Static_assert(sizeof(frame_out) == 336, "frame_out");
_Static_assert(sizeof(convert_param) == 72, "convert_param");
#endif

typedef void (*exception_cb)(unsigned int type, void *user);

#define OK 0
#define E_HANDLE 0x80000000u
#define E_CALLORDER 0x80000003u
#define E_PARAMETER 0x80000004u
#define E_NODATA 0x80000007u
#define E_NOENOUGH_BUF 0x8000000Au
#define E_GC_RANGE 0x80000102u
#define E_GC_ACCESS 0x80000106u
#define E_NOT_FOUND 0x80000109u
#define E_ACCESS_DENIED 0x80000203u
#define E_PIXEL 0x80000500u

#define MONO8 0x01080001u
#define BAYERGB8 0x0108000Au
#define RGB8 0x02180014u
#define BGR8 0x02180015u

typedef struct {
	int open, grabbing;
	float exposure;
	int64_t width, height, packet, outstanding, triggers, resend_timeout;
	uint32_t pixel_format, trigger_mode, trigger_source, frame;
	bool reverse_x;
	char user_id[16];
	exception_cb cb;
	void *user;
} cam;

static device_info camera;
static int camera_taken;

static void init_camera(void) {
	memset(&camera, 0, sizeof camera);
	camera.transport = 1;
	camera.mac_high = 0x0011;
	camera.mac_low = 0x22334455;
	unsigned char *b = camera.special;
	b[8] = 50, b[9] = 1, b[10] = 168, b[11] = 192;
	strcpy((char *)b + 20, "FakeVendor");
	strcpy((char *)b + 52, "FAKE-GIGE");
	strcpy((char *)b + 164, "FAKE0001");
	strcpy((char *)b + 180, "bench");
}

#define CAM(h)              \
	cam *c = (cam *)(h);    \
	if (!c) return E_HANDLE

API unsigned int MV_CC_GetSDKVersion(void) { return 0x04080202; }
API int MV_CC_Initialize(void) { return OK; }
API int MV_CC_Finalize(void) { return OK; }

API int MV_CC_EnumDevices(unsigned int layers, device_list *list) {
	if (!list) return E_PARAMETER;
	memset(list, 0, sizeof *list);
	if (layers & 1) {
		init_camera();
		list->count = 1;
		list->devices[0] = &camera;
	}
	return OK;
}

API bool MV_CC_IsDeviceAccessible(device_info *info, unsigned int mode) {
	(void)mode;
	return info && !camera_taken;
}

API int MV_CC_CreateHandle(void **handle, device_info *info) {
	if (!handle || !info || strcmp((char *)info->special + 164, "FAKE0001") != 0) return E_PARAMETER;
	cam *c = calloc(1, sizeof *c);
	c->exposure = 5000, c->width = 64, c->height = 48, c->packet = 1500;
	c->pixel_format = BAYERGB8;
	*handle = c;
	return OK;
}

// A handle that is never opened, as the real SDK allows for conversion.
API int MV_CC_CreateHandleWithoutLog(void **handle, device_info *info) {
	if (!handle || !info || info->transport != 1) return E_PARAMETER;
	*handle = calloc(1, sizeof(cam));
	return OK;
}

API int MV_CC_DestroyHandle(void *h) {
	CAM(h);
	free(c);
	return OK;
}

API int MV_CC_OpenDevice(void *h, unsigned int mode, unsigned short key) {
	CAM(h);
	(void)mode, (void)key;
	if (camera_taken) return E_ACCESS_DENIED;
	camera_taken = c->open = 1;
	return OK;
}

API int MV_CC_CloseDevice(void *h) {
	CAM(h);
	if (c->open) camera_taken = c->open = 0;
	return OK;
}

API bool MV_CC_IsDeviceConnected(void *h) { return h && ((cam *)h)->open; }
API int MV_CC_GetOptimalPacketSize(void *h) { return h ? 8164 : (int)E_HANDLE; }

API int MV_CC_GetIntValueEx(void *h, const char *key, int_value *v) {
	CAM(h);
	memset(v, 0, sizeof *v);
	v->min = 0, v->max = 1 << 30, v->inc = 1;
	if (!strcmp(key, "Width")) v->cur = c->width, v->inc = 8;
	else if (!strcmp(key, "Height")) v->cur = c->height, v->inc = 2;
	else if (!strcmp(key, "GevSCPSPacketSize")) v->cur = c->packet;
	else if (!strcmp(key, "FakeOutstanding")) v->cur = c->outstanding;
	else if (!strcmp(key, "FakeResendTimeout")) v->cur = c->resend_timeout;
	else return E_NOT_FOUND;
	return OK;
}

API int MV_CC_SetIntValueEx(void *h, const char *key, int64_t value) {
	CAM(h);
	if (!strcmp(key, "Width") || !strcmp(key, "Height")) {
		if (c->grabbing) return E_GC_ACCESS;
		if (value < 2 || value > 4096) return E_GC_RANGE;
		*(key[0] == 'W' ? &c->width : &c->height) = value;
	} else if (!strcmp(key, "GevSCPSPacketSize")) {
		c->packet = value;
	} else {
		return E_NOT_FOUND;
	}
	return OK;
}

API int MV_CC_GetFloatValue(void *h, const char *key, float_value *v) {
	CAM(h);
	if (strcmp(key, "ExposureTime")) return E_NOT_FOUND;
	memset(v, 0, sizeof *v);
	v->cur = c->exposure, v->min = 15, v->max = 1000000;
	return OK;
}

API int MV_CC_SetFloatValue(void *h, const char *key, float value) {
	CAM(h);
	if (strcmp(key, "ExposureTime")) return E_NOT_FOUND;
	if (value < 15 || value > 1000000) return E_GC_RANGE;
	c->exposure = value;
	return OK;
}

typedef struct {
	const char *node, *symbol;
	uint32_t value;
} enum_def;

static const enum_def enums[] = {
	{"PixelFormat", "Mono8", MONO8}, {"PixelFormat", "BayerGB8", BAYERGB8},
	{"TriggerMode", "Off", 0},       {"TriggerMode", "On", 1},
	{"TriggerSource", "Line0", 0},   {"TriggerSource", "Software", 7},
};

static uint32_t *enum_field(cam *c, const char *key) {
	if (!strcmp(key, "PixelFormat")) return &c->pixel_format;
	if (!strcmp(key, "TriggerMode")) return &c->trigger_mode;
	if (!strcmp(key, "TriggerSource")) return &c->trigger_source;
	return NULL;
}

API int MV_CC_GetEnumValue(void *h, const char *key, enum_value *v) {
	CAM(h);
	uint32_t *f = enum_field(c, key);
	if (!f) return E_NOT_FOUND;
	memset(v, 0, sizeof *v);
	v->cur = *f;
	for (size_t i = 0; i < sizeof enums / sizeof *enums; i++)
		if (!strcmp(enums[i].node, key)) v->supported[v->count++] = enums[i].value;
	return OK;
}

API int MV_CC_SetEnumValue(void *h, const char *key, unsigned int value) {
	CAM(h);
	uint32_t *f = enum_field(c, key);
	if (!f) return E_NOT_FOUND;
	for (size_t i = 0; i < sizeof enums / sizeof *enums; i++)
		if (!strcmp(enums[i].node, key) && enums[i].value == value) {
			if (f == &c->pixel_format && c->grabbing) return E_GC_ACCESS;
			*f = value;
			return OK;
		}
	return E_GC_RANGE;
}

API int MV_CC_SetEnumValueByString(void *h, const char *key, const char *symbol) {
	CAM(h);
	for (size_t i = 0; i < sizeof enums / sizeof *enums; i++)
		if (!strcmp(enums[i].node, key) && !strcmp(enums[i].symbol, symbol))
			return MV_CC_SetEnumValue(h, key, enums[i].value);
	return enum_field(c, key) ? E_GC_RANGE : E_NOT_FOUND;
}

API int MV_CC_GetEnumEntrySymbolic(void *h, const char *key, enum_entry *e) {
	CAM(h);
	for (size_t i = 0; i < sizeof enums / sizeof *enums; i++)
		if (!strcmp(enums[i].node, key) && enums[i].value == e->value) {
			strcpy(e->symbol, enums[i].symbol);
			return OK;
		}
	return enum_field(c, key) ? E_GC_RANGE : E_NOT_FOUND;
}

API int MV_CC_GetBoolValue(void *h, const char *key, bool *v) {
	CAM(h);
	if (strcmp(key, "ReverseX")) return E_NOT_FOUND;
	*v = c->reverse_x;
	return OK;
}

API int MV_CC_SetBoolValue(void *h, const char *key, bool v) {
	CAM(h);
	if (strcmp(key, "ReverseX")) return E_NOT_FOUND;
	c->reverse_x = v;
	return OK;
}

API int MV_CC_GetStringValue(void *h, const char *key, string_value *v) {
	CAM(h);
	if (strcmp(key, "DeviceUserID")) return E_NOT_FOUND;
	memset(v, 0, sizeof *v);
	strcpy(v->cur, c->user_id);
	v->max_length = sizeof c->user_id - 1;
	return OK;
}

API int MV_CC_SetStringValue(void *h, const char *key, const char *value) {
	CAM(h);
	if (strcmp(key, "DeviceUserID")) return E_NOT_FOUND;
	if (strlen(value) >= sizeof c->user_id) return E_GC_RANGE;
	strcpy(c->user_id, value);
	return OK;
}

#ifdef _WIN32
static DWORD WINAPI fire(LPVOID arg) {
#else
static void *fire(void *arg) {
#endif
	cam *c = arg;
	c->cb(0x8001, c->user); // device disconnected
	return 0;
}

API int MV_CC_SetCommandValue(void *h, const char *key) {
	CAM(h);
	if (!strcmp(key, "TriggerSoftware")) {
		c->triggers++;
		return OK;
	}
	if (!strcmp(key, "FakeDisconnect")) { // fire the callback from a foreign thread
		if (!c->cb) return E_CALLORDER;
#ifdef _WIN32
		HANDLE t = CreateThread(NULL, 0, fire, c, 0, NULL);
		WaitForSingleObject(t, INFINITE);
		CloseHandle(t);
#else
		pthread_t t;
		pthread_create(&t, NULL, fire, c);
		pthread_join(t, NULL);
#endif
		return OK;
	}
	return E_NOT_FOUND;
}

API int MV_CC_RegisterExceptionCallBack(void *h, exception_cb cb, void *user) {
	CAM(h);
	c->cb = cb, c->user = user;
	return OK;
}

API int MV_CC_FeatureSave(void *h, const char *path) {
	CAM(h);
	FILE *f = fopen(path, "w");
	if (!f) return 0x8000000Fu;
	fprintf(f, "ExposureTime=%f\n", c->exposure);
	fclose(f);
	return OK;
}

API int MV_CC_FeatureLoad(void *h, const char *path) {
	CAM(h);
	FILE *f = fopen(path, "r");
	if (!f) return 0x8000000Fu;
	int n = fscanf(f, "ExposureTime=%f", &c->exposure);
	fclose(f);
	return n == 1 ? OK : E_PARAMETER;
}

API int MV_CC_SetGrabStrategy(void *h, unsigned int s) { return !h ? E_HANDLE : s > 3 ? E_PARAMETER : OK; }
API int MV_CC_SetImageNodeNum(void *h, unsigned int n) { return !h ? E_HANDLE : n == 0 ? E_PARAMETER : OK; }
API int MV_CC_SetOutputQueueSize(void *h, unsigned int n) { return !h ? E_HANDLE : n == 0 ? E_PARAMETER : OK; }
API int MV_CC_ClearImageBuffer(void *h) { return h ? OK : E_HANDLE; }
API int MV_CC_SetBayerCvtQuality(void *h, unsigned int q) { return !h ? E_HANDLE : q > 3 ? E_PARAMETER : OK; }

API int MV_GIGE_SetResend(void *h, unsigned int enable, unsigned int percent, unsigned int timeout) {
	CAM(h);
	if (enable > 1 || percent > 100) return E_PARAMETER;
	c->resend_timeout = enable ? timeout : 0;
	return OK;
}

API int MV_CC_StartGrabbing(void *h) {
	CAM(h);
	if (!c->open || c->grabbing) return E_CALLORDER;
	c->grabbing = 1;
	return OK;
}

API int MV_CC_StopGrabbing(void *h) {
	CAM(h);
	if (!c->grabbing) return E_CALLORDER;
	c->grabbing = 0;
	return OK;
}

// Frames are 1 byte per pixel; pixel (x, y) of frame n holds n + x + y.
API int MV_CC_GetImageBuffer(void *h, frame_out *out, unsigned int timeout) {
	CAM(h);
	(void)timeout;
	if (!c->grabbing) return E_CALLORDER;
	if (c->trigger_mode == 1) {
		if (c->triggers == 0) return E_NODATA;
		c->triggers--;
	}
	size_t size = (size_t)(c->width * c->height);
	memset(out, 0, sizeof *out);
	out->data = malloc(size);
	c->frame++;
	for (int64_t y = 0; y < c->height; y++)
		for (int64_t x = 0; x < c->width; x++) out->data[y * c->width + x] = (unsigned char)(c->frame + x + y);
	frame_info *fi = &out->info;
	fi->width = (uint16_t)c->width, fi->height = (uint16_t)c->height;
	fi->pixel_type = c->pixel_format, fi->number = c->frame;
	fi->dev_time_high = 1, fi->dev_time_low = c->frame;
	fi->host_time = 1700000000000LL + c->frame;
	fi->length = (uint32_t)size, fi->ext_width = (uint32_t)c->width, fi->ext_height = (uint32_t)c->height;
	fi->ext_length = size;
	fi->lost_packets = c->frame % 2;
	c->outstanding++;
	return OK;
}

API int MV_CC_FreeImageBuffer(void *h, frame_out *out) {
	CAM(h);
	if (!out || !out->data) return E_PARAMETER;
	free(out->data);
	out->data = NULL;
	c->outstanding--;
	return OK;
}

// "Demosaics" deterministically: RGB = (v, v/2, 255-v) for Bayer, (v, v, v)
// for Mono8.
API int MV_CC_ConvertPixelTypeEx(void *h, convert_param *p) {
	CAM(h);
	(void)c;
	size_t n = (size_t)p->width * p->height;
	if (p->src_type != MONO8 && p->src_type != BAYERGB8) return E_PIXEL;
	if (p->dst_type != RGB8 && p->dst_type != BGR8) return E_PIXEL;
	if (p->src_len < n || !p->src || !p->dst) return E_PARAMETER;
	if (p->dst_size < n * 3) return E_NOENOUGH_BUF;
	for (size_t i = 0; i < n; i++) {
		unsigned char v = p->src[i];
		unsigned char r = v, g = p->src_type == MONO8 ? v : v / 2, b = p->src_type == MONO8 ? v : 255 - v;
		unsigned char *d = p->dst + 3 * i;
		if (p->dst_type == RGB8) d[0] = r, d[1] = g, d[2] = b;
		else d[0] = b, d[1] = g, d[2] = r;
	}
	p->dst_len = (uint32_t)(n * 3);
	return OK;
}
