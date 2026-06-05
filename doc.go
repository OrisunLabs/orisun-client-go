// Package orisun provides a Go client for the Orisun EventStore and Admin gRPC
// APIs.
//
// The primary entry point is New, which connects to an Orisun server and returns
// a Client. Applications use the client to save events, read event history,
// subscribe to event streams, and call admin APIs.
//
// Generated protobuf request and response types are available from the
// github.com/oexza/orisun-client-go/eventstore package.
package orisun
