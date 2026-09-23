package orisun

import (
	"context"
	"math"
	"net"
	"testing"

	eventstore "github.com/oexza/orisun-client-go/eventstore"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

type writeContextTestServer struct {
	eventstore.UnimplementedEventStoreServer
	context *eventstore.WriteContext
}

func (s writeContextTestServer) GetWriteContext(_ context.Context, req *eventstore.GetWriteContextRequest) (*eventstore.WriteContext, error) {
	if req.Boundary != "orders" || req.WriteId != s.context.WriteId {
		return nil, status.Error(codes.NotFound, "write context not found")
	}
	return s.context, nil
}

func (s writeContextTestServer) SaveEventsV2(_ context.Context, req *eventstore.SaveEventsV2Request) (*eventstore.WriteResult, error) {
	return &eventstore.WriteResult{WriteId: s.context.WriteId, LogPosition: req.Consistency[0].Position}, nil
}

func TestWriteContextRoundTrip(t *testing.T) {
	want := &eventstore.WriteContext{WriteId: "9223372036854775807:7", Consistency: []*eventstore.ConsistencyObservation{{
		Query:    &eventstore.Query{Criteria: []*eventstore.Criterion{{Tags: []*eventstore.Tag{{Key: "__eventType", Value: "Created"}}}}},
		Position: &eventstore.Position{CommitPosition: math.MaxInt64, PreparePosition: 7},
	}}}
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	eventstore.RegisterEventStoreServer(server, writeContextTestServer{context: want})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := &OrisunClient{conn: conn, client: eventstore.NewEventStoreClient(conn), logger: NewDefaultLogger(ERROR)}
	got, err := client.GetWriteContext(t.Context(), &eventstore.GetWriteContextRequest{Boundary: "orders", WriteId: want.WriteId})
	require.NoError(t, err)
	require.True(t, proto.Equal(want, got))
	saved, err := client.SaveEventsV2(t.Context(), &eventstore.SaveEventsV2Request{Boundary: "orders", Consistency: got.Consistency,
		Events: []*eventstore.EventToSave{{EventId: "e1", EventType: "Created", Data: `{}`}},
	})
	require.NoError(t, err)
	require.Equal(t, want.WriteId, saved.WriteId)
	require.Equal(t, int64(math.MaxInt64), saved.LogPosition.CommitPosition)
	_, err = client.GetWriteContext(t.Context(), &eventstore.GetWriteContextRequest{Boundary: "other", WriteId: want.WriteId})
	require.Error(t, err)
	var wrapped *OrisunException
	require.ErrorAs(t, err, &wrapped)
	require.Equal(t, codes.NotFound, status.Code(wrapped.Unwrap()))
	for _, req := range []*eventstore.GetWriteContextRequest{nil, {}, {Boundary: "orders"}, {WriteId: "1:1"}} {
		_, err = client.GetWriteContext(t.Context(), req)
		require.Error(t, err)
	}
}
