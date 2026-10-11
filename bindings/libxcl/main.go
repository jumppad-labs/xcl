// Command libxcl is the xcl engine built as a C shared library, with
// go build -buildmode=c-shared. It exports the four functions of the C ABI
// declared in xcl.h, and nothing else: every operation is one CoreService
// message passed through xcl_call, dispatched by coreservice. The library
// moves bytes between C and coreservice.Service and holds no logic of its
// own; see proto/xcl/core/v1/core.proto for the messages and the coreservice
// package for their behaviour.
//
// Memory crosses the boundary by copying. Request bytes belong to the caller
// and are copied before xcl_call returns; response bytes are allocated with
// C malloc, so the garbage collector never moves them, and the caller frees
// each one exactly once with xcl_free. No Go pointer ever crosses to C.
package main

/*
#include <stdint.h>
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/jumppad-labs/xcl/coreservice"
)

// service holds every configuration a host has created. One per process.
var service = coreservice.New()

// xcl_abi_version returns the C ABI version this library implements. A
// binding compares it with the version it was written against before making
// any other call.
//
//export xcl_abi_version
func xcl_abi_version() C.uint32_t {
	return C.uint32_t(coreservice.ABIVersion)
}

// xcl_call dispatches one CoreService method. handle is a configuration
// handle from CreateConfig, or 0 for a method that needs none. method is the
// method name, NUL terminated. req is the serialised request, read during
// the call only. On return resp holds a buffer the caller frees with
// xcl_free, or NULL when resp_len is 0. It returns a status: 0 success, 1 an
// application error carrying an Error message, and a negative value for a
// programming error in the binding, see xcl.h.
//
//export xcl_call
func xcl_call(handle C.uint64_t, method *C.char, req *C.uint8_t, reqLen C.size_t, resp **C.uint8_t, respLen *C.size_t) C.int32_t {
	var request []byte
	if req != nil && reqLen > 0 {
		request = C.GoBytes(unsafe.Pointer(req), C.int(reqLen))
	}

	out, status := service.Call(uint64(handle), C.GoString(method), request)

	if resp != nil && respLen != nil {
		if len(out) == 0 {
			*resp = nil
			*respLen = 0
		} else {
			*resp = (*C.uint8_t)(C.CBytes(out))
			*respLen = C.size_t(len(out))
		}
	}

	return C.int32_t(status)
}

// xcl_free frees a buffer returned through xcl_call's resp. NULL is ignored.
//
//export xcl_free
func xcl_free(p *C.uint8_t) {
	if p != nil {
		C.free(unsafe.Pointer(p))
	}
}

// xcl_release frees the configuration behind handle, cancelling any
// operation in progress and waiting for it to stop so its state is saved.
//
//export xcl_release
func xcl_release(handle C.uint64_t) C.int32_t {
	return C.int32_t(service.Release(uint64(handle)))
}

func main() {}
