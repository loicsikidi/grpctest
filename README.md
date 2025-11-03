# grpctest

A Go library for testing gRPC servers, inspired by the standard library's [`httptest`](https://pkg.go.dev/net/http/httptest).

## Motivation

When testing gRPC servers in Go, setting up a test server often involves boilerplate code to create a listener, start the server, and manage client connections.`grpctest` simplifies this process by providing utilities to create and manage gRPC test servers with minimal setup, similar to how `httptest` works for HTTP servers.

## Features

Like `httptest`, `grpctest` provides a similar interface for testing gRPC servers:

- **NewServer()**: creates and starts a server on a random local port
- **NewUnstartedServer()**: creates an unstarted server (to be started with Start() or StartTLS())
- **NewTLSServer()**: creates a TLS server with self-signed certificate
- **Server.URL**: contains the server address (e.g., "127.0.0.1:12345")
- **Server.TLS**: server's TLS configuration (i.e. `*tls.Config`)
- **Server.ClientConn()**: returns a configured gRPC client connection to the server
- **Server.Certificate()**: returns the server's x509 certificate (for TLS)

## Installation

```bash
go get github.com/loicsikidi/grpctest
```

## Usage

### Basic example with NewServer

```go
package mypackage_test

import (
    "context"
    "testing"

    "github.com/loicsikidi/grpctest"
    pb "your/proto/package"
    "google.golang.org/grpc"
)

func TestMyGrpcService(t *testing.T) {
    // Create a test server that starts automatically
    server := grpctest.NewServer(func(s *grpc.Server) {
        pb.RegisterYourServiceServer(s, &yourServiceImpl{})
    })
    defer server.Close()

    // server.URL contains the server address (e.g., "127.0.0.1:45123")
    t.Logf("Server listening on: %s", server.URL)

    // Use the client provided by the server
    client := pb.NewYourServiceClient(server.ClientConn())

    // Test your service
    resp, err := client.YourMethod(context.Background(), &pb.YourRequest{})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // Assertions...
}
```

### Example with NewUnstartedServer

```go
func TestWithCustomConfig(t *testing.T) {
    // Create an unstarted server
    server := grpctest.NewUnstartedServer(func(s *grpc.Server) {
        pb.RegisterYourServiceServer(s, &yourServiceImpl{})
    })

    // Configure the server before starting it
    // (e.g., add gRPC options)
    server.Config.ServerOptions = append(
        server.Config.ServerOptions,
        grpc.MaxRecvMsgSize(1024*1024*10),
    )

    // Start the server
    server.Start()
    defer server.Close()

    // Use the server...
}
```

### Example with NewTLSServer

```go
func TestWithTLS(t *testing.T) {
    // Create a TLS server with self-signed certificate
    server := grpctest.NewTLSServer(func(s *grpc.Server) {
        pb.RegisterYourServiceServer(s, &yourServiceImpl{})
    })
    defer server.Close()

    // Verify TLS is configured
    if server.TLS == nil {
        t.Fatal("expected TLS configuration")
    }

    // Get the server's certificate
    cert := server.Certificate()
    t.Logf("Server certificate CN: %s", cert.Subject.CommonName)

    // The client returned by server.ClientConn() automatically
    // trusts the self-signed certificate
    client := pb.NewYourServiceClient(server.ClientConn())

    // Test with TLS...
}
```

### Testing a single method (reducing boilerplate)

When a gRPC service has many methods but you only want to test one, you can use a wrapper that embeds `UnimplementedXXXServer`:

```go
// Create a simple wrapper that embeds UnimplementedGreeterServer
type simpleGreeter struct {
    pb.UnimplementedGreeterServer
    handleFunc func(context.Context, *pb.HelloRequest) (*pb.HelloReply, error)
}

func (s *simpleGreeter) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
    if s.handleFunc != nil {
        return s.handleFunc(ctx, req)
    }
    return &pb.HelloReply{Message: "Hello, " + req.Name + "!"}, nil
}

func TestMinimalImplementation(t *testing.T) {
    server := grpctest.NewServer(func(s *grpc.Server) {
        // Implement only the method you want to test
        pb.RegisterGreeterServer(s, &simpleGreeter{
            handleFunc: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
                if req.Name == "" {
                    t.Error("expected non-empty name")
                }
                return &pb.HelloReply{Message: "Hello, " + req.Name + "!"}, nil
            },
        })
    })
    defer server.Close()

    client := pb.NewGreeterClient(server.ClientConn())
    // Test...
}
```

### Using gRPC interceptors

To add interceptors (logging, auth, tracing, recovery, etc.), use `NewUnstartedServer` to configure the server before starting it:

```go
import (
    "log/slog"
    "os"

    "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
    "github.com/loicsikidi/grpctest"
)

// Helper to adapt slog to go-grpc-middleware format
func InterceptorLogger(l *slog.Logger) logging.Logger {
    return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
        l.Log(ctx, slog.Level(lvl), msg, fields...)
    })
}

func TestWithLoggingInterceptor(t *testing.T) {
    // Create a logger
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    // Create an unstarted server to configure it
    server := grpctest.NewUnstartedServer(func(s *grpc.Server) {
        pb.RegisterGreeterServer(s, &yourServiceImpl{})
    })

    // Add the logging interceptor
    server.Config.ServerOptions = append(
        server.Config.ServerOptions,
        grpc.UnaryInterceptor(
            logging.UnaryServerInterceptor(
                InterceptorLogger(logger),
                logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
            ),
        ),
    )

    // Start the server with configured interceptors
    server.Start()
    defer server.Close()

    // Requests will now be logged
    client := pb.NewGreeterClient(server.ClientConn())
    resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "World"})
    // Logs will show: "started call" and "finished call"
}
```

**Note**: The same pattern works with `StartTLS()` to combine TLS and interceptors.

### Example with request assertions

```go
func TestWithAssertions(t *testing.T) {
    handler := &simpleGreeter{
        handleFunc: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
            // Assertions on the request
            if req.Name == "" {
                t.Error("expected non-empty name")
            }
            if req.Name == "banned" {
                return nil, status.Error(codes.PermissionDenied, "user is banned")
            }

            return &pb.HelloReply{Message: "Hello, " + req.Name + "!"}, nil
        },
    }

    server := grpctest.NewServer(func(s *grpc.Server) {
        pb.RegisterGreeterServer(s, handler)
    })
    defer server.Close()

    client := pb.NewGreeterClient(server.ClientConn())
    ctx := context.Background()

    // Test valid request
    _, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Alice"})
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }

    // Test with validation
    _, err = client.SayHello(ctx, &pb.HelloRequest{Name: ""})
    if status.Code(err) != codes.InvalidArgument {
        t.Errorf("expected InvalidArgument, got %v", status.Code(err))
    }
}
```

## Dependencies

> [!WARNING]
> The project will bump go version as soon as a the latter is dropped from the Go support policy.
> For example, when Go 1.24 is dropped, the project will move to Go 1.25.

| Dependency | Description |
|------------|-------------|
| [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc) | The Go implementation of gRPC, used to create and manage gRPC servers and clients. |
| [google.golang.org/protobuf](https://pkg.go.dev/google.golang.org/protobuf) | Use to build & use [hello](./proto/hello/) proto mainly for testing purpose. **This dep might be removed in the future.** |

## Development

### Prerequisites

This project uses Nix for dependency management. To enter the development environment:

```bash
nix-shell
```

### Generate proto files

> [!NOTE]
> The project uses [`buf`](https://buf.build/docs/cli/quickstart/) to manage and generate protobuf files.

```bash
nix-shell --run "genproto"
```

### Run tests

```bash
nix-shell --run "gotest"
```

## License

BSD-3-Clause License. See the [LICENSE](LICENSE) file for details.

## Disclaimer

This is my personal project and it does not represent my employer. While I do my best to ensure that everything works, I take no responsibility for issues caused by this code.
