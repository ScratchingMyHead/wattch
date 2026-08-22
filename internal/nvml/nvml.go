package nvml

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>

typedef int (*nvmlInit_fn)();
typedef int (*nvmlShutdown_fn)();
typedef int (*nvmlDeviceGetCount_fn)(unsigned int*);
typedef int (*nvmlDeviceGetHandleByIndex_fn)(unsigned int, void**);
typedef int (*nvmlDeviceGetPowerUsage_fn)(void*, unsigned int*);

int call_nvmlInit(void* fn) { return ((nvmlInit_fn)fn)(); }
int call_nvmlShutdown(void* fn) { return ((nvmlShutdown_fn)fn)(); }
int call_nvmlDeviceGetCount(void* fn, unsigned int* c) { return ((nvmlDeviceGetCount_fn)fn)(c); }
int call_nvmlDeviceGetHandleByIndex(void* fn, unsigned int i, void** d) { return ((nvmlDeviceGetHandleByIndex_fn)fn)(i, d); }
int call_nvmlDeviceGetPowerUsage(void* fn, void* d, unsigned int* p) { return ((nvmlDeviceGetPowerUsage_fn)fn)(d, p); }
*/
import "C"
import (
	"fmt"
	"unsafe"
)

var libHandle unsafe.Pointer
var initialized bool

func Init() error {
	if initialized {
		return nil
	}
	cname := C.CString("libnvidia-ml.so.1")
	defer C.free(unsafe.Pointer(cname))
	h := C.dlopen(cname, C.RTLD_LAZY)
	if h == nil {
		errStr := C.GoString(C.dlerror())
		return fmt.Errorf("dlopen libnvidia-ml.so.1: %s", errStr)
	}
	libHandle = h
	// try nvmlInit_v2 then nvmlInit
	csym := C.CString("nvmlInit_v2")
	defer C.free(unsafe.Pointer(csym))
	sym := C.dlsym(h, csym)
	if sym == nil {
		c2 := C.CString("nvmlInit")
		defer C.free(unsafe.Pointer(c2))
		sym = C.dlsym(h, c2)
		if sym == nil {
			C.dlclose(h)
			libHandle = nil
			return fmt.Errorf("nvmlInit not found: %s", C.GoString(C.dlerror()))
		}
	}
	ret := C.call_nvmlInit(sym)
	if ret != 0 {
		C.dlclose(h)
		libHandle = nil
		return fmt.Errorf("nvmlInit failed %d", ret)
	}
	initialized = true
	return nil
}

func Shutdown() {
	if libHandle == nil {
		return
	}
	csym := C.CString("nvmlShutdown")
	defer C.free(unsafe.Pointer(csym))
	sym := C.dlsym(libHandle, csym)
	if sym != nil {
		C.call_nvmlShutdown(sym)
	}
	C.dlclose(libHandle)
	libHandle = nil
	initialized = false
}

func DeviceCount() (int, error) {
	if !initialized {
		return 0, fmt.Errorf("not initialized")
	}
	csym := C.CString("nvmlDeviceGetCount_v2")
	defer C.free(unsafe.Pointer(csym))
	sym := C.dlsym(libHandle, csym)
	if sym == nil {
		c2 := C.CString("nvmlDeviceGetCount")
		defer C.free(unsafe.Pointer(c2))
		sym = C.dlsym(libHandle, c2)
		if sym == nil {
			return 0, fmt.Errorf("nvmlDeviceGetCount not found")
		}
	}
	var cnt C.uint
	ret := C.call_nvmlDeviceGetCount(sym, &cnt)
	if ret != 0 {
		return 0, fmt.Errorf("getCount %d", ret)
	}
	return int(cnt), nil
}

func GetPowerUsage(idx int) (float64, string, error) {
	if !initialized {
		return 0, "", fmt.Errorf("not initialized")
	}
	csym := C.CString("nvmlDeviceGetHandleByIndex_v2")
	defer C.free(unsafe.Pointer(csym))
	sym := C.dlsym(libHandle, csym)
	if sym == nil {
		c2 := C.CString("nvmlDeviceGetHandleByIndex")
		defer C.free(unsafe.Pointer(c2))
		sym = C.dlsym(libHandle, c2)
		if sym == nil {
			return 0, "", fmt.Errorf("GetHandle not found")
		}
	}
	var dev unsafe.Pointer
	ret := C.call_nvmlDeviceGetHandleByIndex(sym, C.uint(idx), &dev)
	if ret != 0 {
		return 0, "", fmt.Errorf("getHandle %d", ret)
	}
	csym2 := C.CString("nvmlDeviceGetPowerUsage")
	defer C.free(unsafe.Pointer(csym2))
	sym2 := C.dlsym(libHandle, csym2)
	if sym2 == nil {
		return 0, "", fmt.Errorf("GetPowerUsage not found")
	}
	var p C.uint
	ret = C.call_nvmlDeviceGetPowerUsage(sym2, dev, &p)
	if ret != 0 {
		return 0, "", fmt.Errorf("getPower %d", ret)
	}
	name := fmt.Sprintf("nvidia:%d", idx)
	return float64(p) / 1000.0, name, nil
}

func Probe() (int, error) {
	if err := Init(); err != nil {
		return 0, err
	}
	cnt, err := DeviceCount()
	if err != nil {
		Shutdown()
		return 0, err
	}
	return cnt, nil
}
