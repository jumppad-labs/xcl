/* libxcl: the xcl engine as a C shared library.
 *
 * Four functions carry the whole API. Every operation is one CoreService
 * message, defined in proto/xcl/core/v1/core.proto, serialised as protobuf
 * and passed through xcl_call with the method's name. A binding generates
 * the message types from the proto and hand-writes only these four
 * declarations.
 *
 * This header documents the ABI; go build -buildmode=c-shared writes an
 * equivalent one beside the library. The ABI version changes only when one of
 * these signatures or a status value changes; adding a CoreService method or
 * message field does not change it.
 */
#ifndef XCL_H
#define XCL_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* The ABI version this library implements. A binding compares it with the
   version it was written against before making any other call. */
uint32_t xcl_abi_version(void);

/* Dispatches one CoreService method.
   handle   a configuration handle from CreateConfig, or 0 for a method that
            needs none (CreateConfig, GetVersion, ConvertTheme).
   method   the CoreService method name, NUL terminated, e.g. "StartApply".
   req      the serialised request message; read during the call only.
   resp     on return, a buffer the caller frees with xcl_free; NULL when
            resp_len is 0.
   Returns a status:
      0   success; resp holds the method's response message
      1   application error; resp holds an Error message
     -1   unknown method
     -2   the request bytes do not parse as the method's request
     -3   unknown or released handle
     -4   the method needs a handle and got 0, or the reverse
   Negative statuses are programming errors in the binding and carry no
   message. */
int32_t xcl_call(uint64_t handle,
                 const char *method,
                 const uint8_t *req, size_t req_len,
                 uint8_t **resp, size_t *resp_len);

/* Frees a buffer returned through resp. NULL is ignored. */
void xcl_free(uint8_t *resp);

/* Frees the configuration behind handle, cancelling any operation in
   progress and waiting for it to stop. Returns a status as xcl_call does. */
int32_t xcl_release(uint64_t handle);

#ifdef __cplusplus
}
#endif

#endif /* XCL_H */
