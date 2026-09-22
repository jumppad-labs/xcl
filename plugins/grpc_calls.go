package plugins

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/jumppad-labs/xcl/logger"
	"google.golang.org/grpc/metadata"
)

// callIDKey is the gRPC metadata key the host sends a provider call's ID
// under. The plugin returns the ID on every log message written during the
// call, so the host can give the message the call's resource, step and
// receiver.
const callIDKey = "xcl-call-id"

// callLoggers holds the logger of every provider call in progress on an
// external plugin, by call ID. It is shared by the host's plugin wrapper,
// which registers each call, and the host callback server, which receives the
// plugin's log messages. It is safe for concurrent use.
type callLoggers struct {
	// nonce keeps the IDs of different hosts apart
	nonce   string
	next    atomic.Uint64
	loggers sync.Map // string -> logger.Logger
}

func newCallLoggers() *callLoggers {
	nonce := make([]byte, 8)
	_, _ = rand.Read(nonce)

	return &callLoggers{nonce: hex.EncodeToString(nonce)}
}

// register records l under a new call ID, returning the ID and a function
// that forgets it once the call has returned
func (c *callLoggers) register(l logger.Logger) (string, func()) {
	id := c.nonce + "-" + strconv.FormatUint(c.next.Add(1), 10)
	c.loggers.Store(id, l)

	return id, func() { c.loggers.Delete(id) }
}

// lookup returns the logger of the call with the given ID, false when no
// such call is in progress
func (c *callLoggers) lookup(id string) (logger.Logger, bool) {
	if c == nil || id == "" {
		return nil, false
	}

	l, ok := c.loggers.Load(id)
	if !ok {
		return nil, false
	}

	return l.(logger.Logger), true
}

// callIDFromContext returns the call ID the host sent with an incoming
// request, empty when there is none
func callIDFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	ids := md.Get(callIDKey)
	if len(ids) == 0 {
		return ""
	}

	return ids[0]
}
