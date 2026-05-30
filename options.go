package orisun

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Option configures a Client.
type Option func(*ClientBuilder)

// New creates a Client for the given gRPC target.
//
// The target can be a normal address such as "localhost:5005" or any target
// accepted by grpc.Dial, such as "dns:///orisun.default.svc.cluster.local:5005".
func New(target string, opts ...Option) (*Client, error) {
	builder := NewClientBuilder().WithTarget(target)
	for _, opt := range opts {
		if opt != nil {
			opt(builder)
		}
	}
	return builder.Build()
}

// WithCredentials configures HTTP Basic credentials.
func WithCredentials(username, password string) Option {
	return func(builder *ClientBuilder) {
		builder.WithBasicAuth(username, password)
	}
}

// WithDefaultTimeout configures the client's default timeout metadata.
func WithDefaultTimeout(timeout time.Duration) Option {
	return func(builder *ClientBuilder) {
		builder.defaultTimeout(timeout)
	}
}

// WithLogger configures a custom logger.
func WithLogger(logger Logger) Option {
	return func(builder *ClientBuilder) {
		builder.WithLogger(logger)
	}
}

// WithLogLevel configures the default logger level.
func WithLogLevel(level LogLevel) Option {
	return func(builder *ClientBuilder) {
		builder.WithLogLevel(level)
	}
}

// WithInsecure disables transport security.
func WithInsecure() Option {
	return func(builder *ClientBuilder) {
		builder.WithTLS(false)
	}
}

// WithTransportCredentials configures gRPC transport credentials.
func WithTransportCredentials(creds credentials.TransportCredentials) Option {
	return func(builder *ClientBuilder) {
		builder.WithTransportCredentials(creds)
	}
}

// WithGRPCDialOptions appends raw gRPC dial options.
func WithGRPCDialOptions(opts ...grpc.DialOption) Option {
	return func(builder *ClientBuilder) {
		builder.WithDialOptions(opts...)
	}
}
