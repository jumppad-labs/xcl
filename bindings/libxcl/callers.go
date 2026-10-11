package main

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"unsafe"
)

// The functions below call the exported C functions through C-typed values,
// as a foreign caller would: the request is copied into C memory, the
// response is read out of the C buffer xcl_call allocated and freed with
// xcl_free. They exist for the smoke test, which cannot import "C" from a
// test file, and are not exported from the library.

// callThroughABI calls xcl_call with a request held in C memory and returns
// the status and a copy of the response, freeing the C response buffer.
func callThroughABI(handle uint64, method string, request []byte) (int32, []byte) {
	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))

	var cRequest *C.uint8_t
	if len(request) > 0 {
		p := C.malloc(C.size_t(len(request)))
		defer C.free(p)

		C.memcpy(p, unsafe.Pointer(&request[0]), C.size_t(len(request)))
		cRequest = (*C.uint8_t)(p)
	}

	var response *C.uint8_t
	var responseLen C.size_t

	status := xcl_call(C.uint64_t(handle), cMethod, cRequest, C.size_t(len(request)), &response, &responseLen)

	var out []byte
	if response != nil {
		out = C.GoBytes(unsafe.Pointer(response), C.int(responseLen))
	}

	xcl_free(response)

	return int32(status), out
}

// releaseThroughABI calls xcl_release.
func releaseThroughABI(handle uint64) int32 {
	return int32(xcl_release(C.uint64_t(handle)))
}

// abiVersionThroughABI calls xcl_abi_version.
func abiVersionThroughABI() uint32 {
	return uint32(xcl_abi_version())
}
