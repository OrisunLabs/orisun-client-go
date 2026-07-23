package orisun

import (
	"context"
	"net"
	"testing"

	eventstore "github.com/oexza/orisun-client-go/eventstore"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestClient_BoundaryManagement_RoundTrip(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	eventstore.RegisterAdminServer(server, boundaryAdminTestServer{})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	client := &OrisunClient{
		conn:        conn,
		adminClient: eventstore.NewAdminClient(conn),
		logger:      NewDefaultLogger(ERROR),
	}
	placement := &eventstore.BoundaryPlacementInput{Backend: "postgres", Namespace: "orders"}

	created, err := client.CreateBoundary(t.Context(), &eventstore.CreateBoundaryRequest{
		Name: "orders", Placement: placement,
	})
	require.NoError(t, err)
	require.Equal(t, eventstore.BoundaryRegistrationOrigin_BOUNDARY_REGISTRATION_ORIGIN_CREATED, created.Boundary.Origin)

	imported, err := client.ImportBoundary(t.Context(), &eventstore.ImportBoundaryRequest{
		Name: "legacy_orders", Placement: placement,
	})
	require.NoError(t, err)
	require.Equal(t, eventstore.BoundaryRegistrationOrigin_BOUNDARY_REGISTRATION_ORIGIN_IMPORTED, imported.Boundary.Origin)

	listed, err := client.ListBoundaries(t.Context(), &eventstore.ListBoundariesRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Boundaries, 2)

	got, err := client.GetBoundary(t.Context(), &eventstore.GetBoundaryRequest{Name: "orders"})
	require.NoError(t, err)
	require.Equal(t, eventstore.BoundaryLifecycleStatus_BOUNDARY_LIFECYCLE_STATUS_ACTIVE, got.Boundary.Status)
}

type boundaryAdminTestServer struct {
	eventstore.UnimplementedAdminServer
}

func (boundaryAdminTestServer) CreateBoundary(
	_ context.Context,
	request *eventstore.CreateBoundaryRequest,
) (*eventstore.CreateBoundaryResponse, error) {
	return &eventstore.CreateBoundaryResponse{Boundary: boundaryInfo(
		request.Name,
		request.Placement,
		eventstore.BoundaryRegistrationOrigin_BOUNDARY_REGISTRATION_ORIGIN_CREATED,
		eventstore.BoundaryLifecycleStatus_BOUNDARY_LIFECYCLE_STATUS_PROVISIONING,
	)}, nil
}

func (boundaryAdminTestServer) ImportBoundary(
	_ context.Context,
	request *eventstore.ImportBoundaryRequest,
) (*eventstore.ImportBoundaryResponse, error) {
	return &eventstore.ImportBoundaryResponse{Boundary: boundaryInfo(
		request.Name,
		request.Placement,
		eventstore.BoundaryRegistrationOrigin_BOUNDARY_REGISTRATION_ORIGIN_IMPORTED,
		eventstore.BoundaryLifecycleStatus_BOUNDARY_LIFECYCLE_STATUS_PROVISIONING,
	)}, nil
}

func (boundaryAdminTestServer) ListBoundaries(
	context.Context,
	*eventstore.ListBoundariesRequest,
) (*eventstore.ListBoundariesResponse, error) {
	placement := &eventstore.BoundaryPlacementInput{Backend: "postgres", Namespace: "orders"}
	return &eventstore.ListBoundariesResponse{Boundaries: []*eventstore.BoundaryInfo{
		boundaryInfo(
			"legacy_orders",
			placement,
			eventstore.BoundaryRegistrationOrigin_BOUNDARY_REGISTRATION_ORIGIN_IMPORTED,
			eventstore.BoundaryLifecycleStatus_BOUNDARY_LIFECYCLE_STATUS_ACTIVE,
		),
		boundaryInfo(
			"orders",
			placement,
			eventstore.BoundaryRegistrationOrigin_BOUNDARY_REGISTRATION_ORIGIN_CREATED,
			eventstore.BoundaryLifecycleStatus_BOUNDARY_LIFECYCLE_STATUS_ACTIVE,
		),
	}}, nil
}

func (boundaryAdminTestServer) GetBoundary(
	_ context.Context,
	request *eventstore.GetBoundaryRequest,
) (*eventstore.GetBoundaryResponse, error) {
	return &eventstore.GetBoundaryResponse{Boundary: boundaryInfo(
		request.Name,
		&eventstore.BoundaryPlacementInput{Backend: "postgres", Namespace: "orders"},
		eventstore.BoundaryRegistrationOrigin_BOUNDARY_REGISTRATION_ORIGIN_CREATED,
		eventstore.BoundaryLifecycleStatus_BOUNDARY_LIFECYCLE_STATUS_ACTIVE,
	)}, nil
}

func boundaryInfo(
	name string,
	placement *eventstore.BoundaryPlacementInput,
	origin eventstore.BoundaryRegistrationOrigin,
	status eventstore.BoundaryLifecycleStatus,
) *eventstore.BoundaryInfo {
	return &eventstore.BoundaryInfo{
		Name:      name,
		Placement: placement,
		Origin:    origin,
		Status:    status,
	}
}
