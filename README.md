# Orisun Go Client

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

The older `NewClientBuilder` API remains available for compatibility, but new code should prefer `New` with options.

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
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  protos/*.proto
mv protos/*.go eventstore/
```

## Test

```bash
go test ./...
```

## License

MIT
