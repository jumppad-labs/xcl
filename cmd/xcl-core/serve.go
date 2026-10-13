package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/jumppad-labs/xcl/coreservice"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// releaseMethod is the method name the frame protocol reserves for freeing a
// handle, as xcl_release does. It is not a CoreService method.
const releaseMethod = "Release"

// maxFrameSize bounds the length a frame may declare. A larger length is a
// framing error rather than an allocation of whatever the prefix says.
const maxFrameSize = 256 << 20

// frameHeaderSize is the size of a frame's big-endian length prefix.
const frameHeaderSize = 4

// serve reads Request frames from in until it ends or ctx is cancelled,
// dispatches each on its own goroutine to service, and writes Response
// frames to out, one at a time, so a Poll that waits never holds up another
// call. Before returning it releases every configuration (service.Close),
// which saves the state of any operation in progress, and waits for every
// request in flight to be answered.
//
// End of input, cancelling ctx and a failed write (the host has gone) all
// return nil. A truncated frame, a frame longer than maxFrameSize or a frame
// that does not decode as a Request returns an error.
func serve(ctx context.Context, in io.Reader, out io.Writer, service *coreservice.Service) error {
	server := &server{service: service, out: out, writeFailed: make(chan struct{})}

	done := make(chan struct{})
	defer close(done)

	frames := make(chan frame)
	go readFrames(in, frames, done)

	var result error

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-server.writeFailed:
			break loop
		case next := <-frames:
			if next.err != nil {
				if !errors.Is(next.err, io.EOF) {
					result = next.err
				}
				break loop
			}

			server.dispatch(next.request)
		}
	}

	server.stop()

	return result
}

// frame is one decoded Request, or the error that ended the input.
type frame struct {
	request *corepb.Request
	err     error
}

// readFrames decodes Request frames from in and sends each on frames until
// the input ends, sending that error last: io.EOF for a clean end between
// frames. It stops early when done is closed.
func readFrames(in io.Reader, frames chan<- frame, done <-chan struct{}) {
	reader := bufio.NewReader(in)

	for {
		request, err := readRequest(reader)

		select {
		case frames <- frame{request: request, err: err}:
		case <-done:
			return
		}

		if err != nil {
			return
		}
	}
}

// readRequest reads one length-prefixed Request. It returns io.EOF when the
// input ends cleanly before a frame starts.
func readRequest(reader io.Reader) (*corepb.Request, error) {
	var header [frameHeaderSize]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}

		return nil, fmt.Errorf("truncated frame header: %w", err)
	}

	size := binary.BigEndian.Uint32(header[:])
	if size > maxFrameSize {
		return nil, fmt.Errorf("frame of %d bytes is larger than the %d byte limit", size, maxFrameSize)
	}

	body := make([]byte, size)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, fmt.Errorf("truncated frame: expected %d bytes: %w", size, err)
	}

	request := &corepb.Request{}
	if err := proto.Unmarshal(body, request); err != nil {
		return nil, fmt.Errorf("frame is not a Request: %w", err)
	}

	return request, nil
}

// server answers requests concurrently and serialises their responses.
type server struct {
	service *coreservice.Service

	writeMu     sync.Mutex
	out         io.Writer
	writeErr    error
	writeFailed chan struct{}

	// stopping refuses new requests once stop has begun, so the in-flight
	// group is never added to while it is being waited on
	mu       sync.Mutex
	stopping bool
	inFlight sync.WaitGroup
}

// dispatch answers request on its own goroutine. A request that arrives once
// the server is stopping is dropped: its host is going away.
func (s *server) dispatch(request *corepb.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopping {
		return
	}

	s.inFlight.Go(func() {
		s.answer(request)
	})
}

// answer runs request against the service and writes its response.
func (s *server) answer(request *corepb.Request) {
	var payload []byte
	var status coreservice.Status

	if request.GetMethod() == releaseMethod {
		status = s.service.Release(request.GetHandle())
	} else {
		payload, status = s.service.Call(request.GetHandle(), request.GetMethod(), request.GetPayload())
	}

	s.write(&corepb.Response{
		Id: request.GetId(),
		// the negative statuses cross as their two's-complement value
		Status:  uint32(int32(status)),
		Payload: payload,
	})
}

// write sends one length-prefixed Response. After the first failed write,
// which means the host has gone, every later response is dropped.
func (s *server) write(response *corepb.Response) {
	body, err := proto.Marshal(response)
	if err != nil {
		// a Response of scalars and bytes always marshals
		panic(fmt.Sprintf("marshalling a Response: %v", err))
	}

	message := make([]byte, frameHeaderSize+len(body))
	binary.BigEndian.PutUint32(message, uint32(len(body)))
	copy(message[frameHeaderSize:], body)

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.writeErr != nil {
		return
	}

	if _, err := s.out.Write(message); err != nil {
		s.writeErr = err
		close(s.writeFailed)
	}
}

// stop releases every configuration, which ends any Poll waiting on one and
// saves the state of any operation in progress, then waits for every request
// in flight to be answered. A configuration created by a request that was
// still in flight during the first release is released by the second.
func (s *server) stop() {
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()

	s.service.Close()
	s.inFlight.Wait()
	s.service.Close()
}
