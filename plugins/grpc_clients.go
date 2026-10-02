package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jumppad-labs/xcl/logger"
	"strconv"

	"github.com/jumppad-labs/xcl/plugins/proto"
)

// GRPCLogger sends a plugin's log messages to the host, over the host
// callback service. callID is the ID of the provider call the logger was made
// for, the host uses it to give each message the call's resource, step and
// receiver. It is empty for the plugin scoped logger.
//
// A message that fails to send is dropped: logging must never change the
// outcome of a provider call.
type GRPCLogger struct {
	client proto.HostCallbackServiceClient
	callID string
}

// Ensure GRPCLogger implements logger.Logger interface
var _ logger.Logger = (*GRPCLogger)(nil)

func (l *GRPCLogger) request(msg string, args []any) *proto.LogRequest {
	return &proto.LogRequest{
		Message: msg,
		Args:    interfaceArgsToStrings(args),
		CallId:  l.callID,
	}
}

func (l *GRPCLogger) Info(msg string, args ...any) {
	_, _ = l.client.Info(context.Background(), l.request(msg, args))
}

func (l *GRPCLogger) Debug(msg string, args ...any) {
	_, _ = l.client.Debug(context.Background(), l.request(msg, args))
}

func (l *GRPCLogger) Warn(msg string, args ...any) {
	_, _ = l.client.Warn(context.Background(), l.request(msg, args))
}

func (l *GRPCLogger) Error(msg string, args ...any) {
	_, _ = l.client.Error(context.Background(), l.request(msg, args))
}

// asyncLogger is the plugin scoped logger inside an external plugin process,
// for messages written outside a provider call, such as in Init. Init runs
// before the host has connected, so each message is sent from its own
// goroutine once the connection to the host is available, rather than
// holding up the plugin's start. Messages sent this way may arrive out of
// order, and are dropped when the host cannot be reached.
type asyncLogger struct {
	client func() (proto.HostCallbackServiceClient, error)
}

// Ensure asyncLogger implements logger.Logger interface
var _ logger.Logger = (*asyncLogger)(nil)

func (l *asyncLogger) send(log func(l *GRPCLogger)) {
	go func() {
		client, err := l.client()
		if err != nil {
			return
		}

		log(&GRPCLogger{client: client})
	}()
}

func (l *asyncLogger) Info(msg string, args ...any) {
	l.send(func(g *GRPCLogger) { g.Info(msg, args...) })
}

func (l *asyncLogger) Debug(msg string, args ...any) {
	l.send(func(g *GRPCLogger) { g.Debug(msg, args...) })
}

func (l *asyncLogger) Warn(msg string, args ...any) {
	l.send(func(g *GRPCLogger) { g.Warn(msg, args...) })
}

func (l *asyncLogger) Error(msg string, args ...any) {
	l.send(func(g *GRPCLogger) { g.Error(msg, args...) })
}

// GRPCState implements State interface using consolidated HostCallbackService client
type GRPCState struct {
	client proto.HostCallbackServiceClient
}

// Ensure GRPCState implements State interface
var _ State = (*GRPCState)(nil)

func (s *GRPCState) Get(key string) (any, error) {
	resp, err := s.client.Get(context.Background(), &proto.StateGetRequest{Key: key})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	// Unmarshal the resource data back to any
	// This is a simplified implementation - in practice you'd need type information
	// to properly unmarshal to the correct concrete type
	var resource map[string]interface{}
	if err := json.Unmarshal(resp.ResourceData, &resource); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	// TODO: This needs to be enhanced to return proper typed resources
	// For now, returning nil as this is a complex cross-process serialization issue
	return nil, fmt.Errorf("resource deserialization not implemented")
}

func (s *GRPCState) Find(pattern string) ([]any, error) {
	resp, err := s.client.Find(context.Background(), &proto.StateFindRequest{Pattern: pattern})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	// Similar issue as Get - need to properly deserialize resources
	// TODO: Implement proper resource deserialization
	return nil, fmt.Errorf("resource deserialization not implemented")
}

// Helper functions
func interfaceArgsToStrings(args []any) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			result[i] = v
		case int:
			result[i] = strconv.Itoa(v)
		case float64:
			result[i] = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			result[i] = strconv.FormatBool(v)
		default:
			result[i] = fmt.Sprintf("%v", v)
		}
	}
	return result
}
