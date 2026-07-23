# Orisun Go Client

[![Go Reference](https://pkg.go.dev/badge/github.com/oexza/orisun-client-go.svg)](https://pkg.go.dev/github.com/oexza/orisun-client-go)

Idiomatic Go client for the Orisun EventStore and Admin gRPC APIs.

## Install

```bash
go get github.com/oexza/orisun-client-go
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"time"

	orisun "github.com/oexza/orisun-client-go"
)

func main() {
	client, err := orisun.New(
		"localhost:5005",
		orisun.WithCredentials("admin", "changeit"),
		orisun.WithDefaultTimeout(30*time.Second),
		orisun.WithInsecure(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		log.Fatal(err)
	}
}
```

`New` accepts any gRPC target accepted by `grpc.Dial`, including `localhost:5005` and DNS targets such as `dns:///orisun.default.svc.cluster.local:5005`.

## Saving Events

```go
package main

import (
	"context"

	orisun "github.com/oexza/orisun-client-go"
	eventstore "github.com/oexza/orisun-client-go/eventstore"
)

func save(ctx context.Context, client *orisun.Client) error {
	_, err := client.SaveEvents(ctx, &eventstore.SaveEventsRequest{
		Boundary: "orders",
		Query: &eventstore.SaveQuery{
			ExpectedPosition: &eventstore.Position{CommitPosition: -1, PreparePosition: -1},
		},
		Events: []*eventstore.EventToSave{
			{
				EventId:   "order-001",
				EventType: "OrderPlaced",
				Data:      `{"customer_id":"c-1","amount":45}`,
				Metadata:  `{"source":"checkout"}`,
			},
		},
	})
	return err
}
```

Event IDs are application-defined stable strings. Use IDs that make retry and deduplication safe for your application.

## Reading Events

```go
resp, err := client.GetEvents(ctx, &eventstore.GetEventsRequest{
	Boundary:  "orders",
	Count:     100,
	Direction: eventstore.Direction_ASC,
})
if err != nil {
	return err
}

for _, event := range resp.Events {
	// event.Position identifies durable ordering within the boundary.
	_ = event
}
```

## Reading Latest Context State

For carried-state command contexts, use `GetLatestByCriteria` to read the latest event for each criterion from one server-side snapshot. Use the returned `ContextPosition` as the next `SaveEvents.Query.ExpectedPosition` with the same combined criteria.

```go
resp, err := client.GetLatestByCriteria(ctx, &eventstore.GetLatestByCriteriaRequest{
	Boundary: "accounts",
	Criteria: []*eventstore.Criterion{
		{
			Tags: []*eventstore.Tag{
				{Key: "eventType", Value: "AccountOpened"},
				{Key: "accountOpenedId", Value: "018f2d5e-2001-7000-8000-000000000001"},
			},
		},
		{
			Tags: []*eventstore.Tag{
				{Key: "scopes.accountOpenedId", Value: "018f2d5e-2001-7000-8000-000000000001"},
			},
		},
		{
			Tags: []*eventstore.Tag{
				{Key: "eventType", Value: "AccountOpened"},
				{Key: "accountOpenedId", Value: "018f2d5e-2002-7000-8000-000000000002"},
			},
		},
		{
			Tags: []*eventstore.Tag{
				{Key: "scopes.accountOpenedId", Value: "018f2d5e-2002-7000-8000-000000000002"},
			},
		},
	},
})
if err != nil {
	return err
}

expectedPosition := resp.ContextPosition
_ = expectedPosition
```

## Subscriptions

```go
handler := orisun.NewSimpleEventHandler().
	WithOnEvent(func(event *eventstore.Event) error {
		// Persist side effects, then checkpoint event.Position in your application.
		return nil
	}).
	WithOnError(func(err error) {
		log.Printf("subscription stopped: %v", err)
	})

sub, err := client.SubscribeToEvents(ctx, &eventstore.CatchUpSubscribeToEventStoreRequest{
	Boundary:       "orders",
	SubscriberName: "orders-projector",
	AfterPosition:  &eventstore.Position{CommitPosition: -1, PreparePosition: -1},
}, handler)
if err != nil {
	return err
}
defer sub.Close()
```

Subscriptions catch up from durable storage and then switch to live delivery. Delivery is at least once, so consumers should deduplicate by `event_id` or idempotent writes.

## Options

```go
client, err := orisun.New(
	"localhost:5005",
	orisun.WithCredentials("admin", "changeit"),
	orisun.WithDefaultTimeout(30*time.Second),
	orisun.WithLogLevel(orisun.INFO),
	orisun.WithMaxReceiveMessageSize(100*1024*1024),
	orisun.WithMaxSendMessageSize(100*1024*1024),
	orisun.WithFlowControlWindow(1024*1024),
	orisun.WithInsecure(),
)
```

Available options:

- `WithCredentials(username, password)` sends Basic auth until the server returns an auth token; the client reuses that token on later calls.
- `WithDefaultTimeout(duration)` records the default timeout used by helper code.
- `WithLogger(logger)` sets a custom logger.
- `WithLogLevel(level)` configures the default logger.
- `WithInsecure()` uses plaintext transport for local development.
- `WithTransportCredentials(creds)` sets explicit gRPC transport credentials.
- `WithGRPCDialOptions(opts...)` appends raw gRPC dial options.
- `WithMaxReceiveMessageSize(bytes)` overrides the maximum inbound gRPC message size. The default is 100 MB.
- `WithMaxSendMessageSize(bytes)` overrides the maximum outbound gRPC message size. The default is 100 MB.
- `WithFlowControlWindow(bytes)` overrides the HTTP/2 stream and connection flow-control window. The default is 1 MB.

The older `NewClientBuilder` API remains available for compatibility, but new code should prefer `New` with options.

## Boundary Management

Boundary definitions are durable admin commands. Create and import calls return
the catalog entry in `PROVISIONING`; wait for `ACTIVE` before using the boundary
with event-store APIs.

```go
placement := &eventstore.BoundaryPlacementInput{
	Backend:   "postgres",
	Namespace: "orders",
}

created, err := client.CreateBoundary(ctx, &eventstore.CreateBoundaryRequest{
	Name:        "orders",
	Description: "Order lifecycle events",
	Placement:   placement,
})
if err != nil {
	log.Fatal(err)
}

boundary := created.Boundary
for boundary.Status == eventstore.BoundaryLifecycleStatus_BOUNDARY_LIFECYCLE_STATUS_PROVISIONING {
	time.Sleep(100 * time.Millisecond)
	response, err := client.GetBoundary(ctx, &eventstore.GetBoundaryRequest{Name: "orders"})
	if err != nil {
		log.Fatal(err)
	}
	boundary = response.Boundary
}

catalog, err := client.ListBoundaries(ctx, &eventstore.ListBoundariesRequest{})
```

Use `ImportBoundary` instead when the physical boundary already exists. Both
operations reject duplicate names with gRPC `ALREADY_EXISTS`.

## High-Throughput Writes

Use one client per target and reuse it. The client caches Orisun auth tokens after the first authenticated response, so hot `SaveEvents` calls should use the cached token path rather than repeatedly sending Basic credentials.

For burst imports or very high write volume, keep roughly 512-1024 `SaveEvents` calls in flight instead of launching an unbounded number of goroutines. The server can process large bursts, but flooding one HTTP/2 connection with every pending write at once adds client-side scheduling and stream overhead.

```go
sem := make(chan struct{}, 1024)
var wg sync.WaitGroup

for _, req := range requests {
	wg.Add(1)
	sem <- struct{}{}
	go func(req *eventstore.SaveEventsRequest) {
		defer wg.Done()
		defer func() { <-sem }()
		_, _ = client.SaveEvents(ctx, req)
	}(req)
}

wg.Wait()
```

## Errors

Client methods return ordinary Go `error` values. Use `errors.As` to inspect Orisun-specific errors:

```go
var conflict *orisun.OptimisticConcurrencyException
if errors.As(err, &conflict) {
	log.Printf("consistency conflict: expected=%d actual=%d", conflict.ExpectedVersion(), conflict.ActualVersion())
}

var clientErr *orisun.OrisunException
if errors.As(err, &clientErr) {
	log.Printf("operation failed: %s", clientErr.Message())
}
```

## Generated Protobufs

The generated protobuf package is available at:

```go
import eventstore "github.com/oexza/orisun-client-go/eventstore"
```

To regenerate protobuf files from the `protos` submodule:

```bash
git submodule update --init --recursive
protoc -I protos \
  --go_out=eventstore --go_opt=paths=source_relative \
  '--go_opt=Madmin.proto=github.com/oexza/orisun-client-go/eventstore;orisun' \
  '--go_opt=Meventstore.proto=github.com/oexza/orisun-client-go/eventstore;orisun' \
  --go-grpc_out=eventstore --go-grpc_opt=paths=source_relative \
  '--go-grpc_opt=Madmin.proto=github.com/oexza/orisun-client-go/eventstore;orisun' \
  '--go-grpc_opt=Meventstore.proto=github.com/oexza/orisun-client-go/eventstore;orisun' \
  protos/eventstore.proto protos/admin.proto
```

## Test

```bash
go test ./...
```

## Release

Go packages are published by pushing a semantic version tag to the module's Git
repository. After the tag is public, `pkg.go.dev` indexes the package from
GitHub.

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

Users can install a specific release with:

```bash
go get github.com/oexza/orisun-client-go@vX.Y.Z
```

## License

MIT
