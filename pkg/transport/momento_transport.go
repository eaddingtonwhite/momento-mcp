package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/metoro-io/mcp-golang/transport"
	"github.com/momentohq/client-sdk-go/momento"
)

type RunMode string

var (
	Server RunMode = "server"
	Client RunMode = "client"
)

type MomentoServerTransport struct {
	started            bool
	momentoTopicClient momento.TopicClient
	cacheName          string
	runMode            RunMode
	onClose            func()
	onError            func(error)
	onMessage          func(ctx context.Context, message *transport.BaseJsonRpcMessage)
}

// NewMomentoServerTransport creates a new Momento ServerTransport using topics
func NewMomentoServerTransport(client momento.TopicClient, cacheName string) *MomentoServerTransport {
	return &MomentoServerTransport{
		momentoTopicClient: client,
		cacheName:          cacheName,
		runMode:            Server,
	}
}

// NewMomentoClientTransport creates a new Momento ServerTransport using topics
func NewMomentoClientTransport(client momento.TopicClient, cacheName string) *MomentoServerTransport {
	return &MomentoServerTransport{
		momentoTopicClient: client,
		cacheName:          cacheName,
		runMode:            Client,
	}
}

// Start begins listening for messages on a topic
func (t *MomentoServerTransport) Start(ctx context.Context) error {
	if t.started {
		return fmt.Errorf("MomentoServerTransport already started")
	}
	t.started = true

	go t.readLoop(ctx)
	return nil
}

// Close stops the transport and cleans up resources
func (t *MomentoServerTransport) Close() error {
	t.started = false
	t.momentoTopicClient.Close()
	return nil
}

// Send sends a JSON-RPC message on a topic
func (t *MomentoServerTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	data, err := json.Marshal(message)

	topicName := t.getSendTopicName()
	// TODO make this use configurable debug logging
	//log.Println(fmt.Sprintf(
	//	"sending message to momento on topic %s msg: %s",
	//	topicName, string(data),
	//))
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = t.momentoTopicClient.Publish(ctx, &momento.TopicPublishRequest{
		CacheName: t.cacheName,
		TopicName: topicName,
		Value:     momento.String(data), // TODO send as strings for now for demo. This should be bytes.
	})
	return err
}

func (t *MomentoServerTransport) getSendTopicName() string {
	if t.runMode == Server {
		return "mcp-client"
	}
	return "mcp-server"
}
func (t *MomentoServerTransport) getReadTopicName() string {
	if t.runMode == Server {
		return "mcp-server"
	}
	return "mcp-client"
}

// SetCloseHandler sets the handler for close events
func (t *MomentoServerTransport) SetCloseHandler(handler func()) {
	t.onClose = handler
}

// SetErrorHandler sets the handler for error events
func (t *MomentoServerTransport) SetErrorHandler(handler func(error)) {
	t.onError = handler
}

// SetMessageHandler sets the handler for incoming messages
func (t *MomentoServerTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.onMessage = handler
}

func (t *MomentoServerTransport) readLoop(ctx context.Context) {
	sub, err := t.momentoTopicClient.Subscribe(ctx, &momento.TopicSubscribeRequest{
		CacheName: t.cacheName,
		TopicName: t.getReadTopicName(),
	})
	if err != nil {
		t.handleError(err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			t.Close()
			return
		default:
			if !t.started {
				return
			}
			t.processReadBuffer(ctx, sub)
		}
	}
}

func (t *MomentoServerTransport) processReadBuffer(ctx context.Context, sub momento.TopicSubscription) {
	for {
		v, err := sub.Item(ctx)
		if err != nil {
			t.handleError(err)
			return
		}
		switch v := v.(type) {
		case momento.String: // TODO send as strings for now for demo. This should be bytes.
			if v == "" {
				return
			}
			msg, err := deserializeMessage([]byte(v))
			if err != nil {
				t.handleError(fmt.Errorf("read error: %w", err))
				return
			}
			// TODO make this use configurable/smarter debug logging
			//s, err := msg.MarshalJSON()
			//if err != nil {
			//	t.handleError(fmt.Errorf("read error: %w", err))
			//}
			//
			//log.Println(fmt.Sprintf(
			//	"handling message on topic: %s type:%s msg: %+v",
			//	t.getReadTopicName(), msg.Type, string(s),
			//))
			t.onMessage(ctx, msg)
		}

	}
}

// deserializeMessage deserializes a JSON-RPC message.
// TODO this could be done better copied out of base lib for now.
func deserializeMessage(item []byte) (*transport.BaseJsonRpcMessage, error) {
	var req transport.BaseJSONRPCRequest
	if err := json.Unmarshal(item, &req); err == nil {
		return transport.NewBaseMessageRequest(&req), nil
	}

	var notif transport.BaseJSONRPCNotification
	if err := json.Unmarshal(item, &notif); err == nil {
		return transport.NewBaseMessageNotification(&notif), nil
	}

	var resp transport.BaseJSONRPCResponse
	if err := json.Unmarshal(item, &resp); err == nil {
		return transport.NewBaseMessageResponse(&resp), nil
	}

	var errResp transport.BaseJSONRPCError
	if err := json.Unmarshal(item, &errResp); err == nil {
		return transport.NewBaseMessageError(&errResp), nil
	}

	return nil, errors.New("failed to unmarshal JSON-RPC message, unrecognized type")
}

func (t *MomentoServerTransport) handleError(err error) {
	handler := t.onError

	if handler != nil {
		handler(err)
	}
}

func (t *MomentoServerTransport) handleMessage(msg *transport.BaseJsonRpcMessage) {
	handler := t.onMessage

	ctx := context.Background()

	if handler != nil {
		handler(ctx, msg)
	}
}
