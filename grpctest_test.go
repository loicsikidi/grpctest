package grpctest_test

import (
	"context"
	"testing"
	"time"

	"github.com/loicsikidi/grpctest"
	pb "github.com/loicsikidi/grpctest/proto/hello"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// greeterHandler implements the Greeter service for testing.
type greeterHandler struct {
	pb.UnimplementedGreeterServer
	handler func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error)
}

func (h *greeterHandler) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	if h.handler != nil {
		return h.handler(ctx, req)
	}
	return &pb.HelloReply{Message: "Hello " + req.Name}, nil
}

func TestNewServer(t *testing.T) {
	handler := &greeterHandler{
		handler: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
			return &pb.HelloReply{Message: "Hello " + req.Name}, nil
		},
	}

	server := grpctest.NewServer(func(s *grpc.Server) {
		pb.RegisterGreeterServer(s, handler)
	})
	defer server.Close()

	// Verify server URL is set
	if server.URL == "" {
		t.Fatal("server.URL is empty")
	}
	t.Logf("Server URL: %s", server.URL)

	// Test using Client() method
	client := pb.NewGreeterClient(server.ClientConn())

	ctx := context.Background()
	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "World"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", resp.Message)
	}
}

func TestNewUnstartedServer(t *testing.T) {
	handler := &greeterHandler{
		handler: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
			return &pb.HelloReply{Message: "Hello " + req.Name}, nil
		},
	}

	server := grpctest.NewUnstartedServer(func(s *grpc.Server) {
		pb.RegisterGreeterServer(s, handler)
	})

	// Server should not be started yet
	if server.URL != "" {
		t.Errorf("expected empty URL before start, got %s", server.URL)
	}

	// Start the server
	server.Start()
	defer server.Close()

	// Verify server URL is set after start
	if server.URL == "" {
		t.Fatal("server.URL is empty after start")
	}
	t.Logf("Server URL: %s", server.URL)

	// Test using Client() method
	client := pb.NewGreeterClient(server.ClientConn())

	ctx := context.Background()
	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "Hello Test" {
		t.Errorf("expected 'Hello Test', got '%s'", resp.Message)
	}
}

func TestNewTLSServer(t *testing.T) {
	handler := &greeterHandler{
		handler: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
			return &pb.HelloReply{Message: "Hello " + req.Name}, nil
		},
	}

	server := grpctest.NewTLSServer(func(s *grpc.Server) {
		pb.RegisterGreeterServer(s, handler)
	})
	defer server.Close()

	// Verify TLS is configured
	if server.TLS == nil {
		t.Fatal("server.TLS is nil")
	}

	// Verify certificate is available
	cert := server.Certificate()
	if cert == nil {
		t.Fatal("server certificate is nil")
	}
	t.Logf("Server certificate subject: %s", cert.Subject.CommonName)

	// Test using Client() method (which should trust the self-signed cert)
	client := pb.NewGreeterClient(server.ClientConn())

	ctx := context.Background()
	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "TLS"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "Hello TLS" {
		t.Errorf("expected 'Hello TLS', got '%s'", resp.Message)
	}
}

func TestStartTLS(t *testing.T) {
	handler := &greeterHandler{
		handler: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
			return &pb.HelloReply{Message: "Hello " + req.Name}, nil
		},
	}

	server := grpctest.NewUnstartedServer(func(s *grpc.Server) {
		pb.RegisterGreeterServer(s, handler)
	})

	// Start with TLS
	server.StartTLS()
	defer server.Close()

	// Verify TLS is configured
	if server.TLS == nil {
		t.Fatal("server.TLS is nil")
	}

	// Verify certificate
	cert := server.Certificate()
	if cert == nil {
		t.Fatal("server certificate is nil")
	}

	// Test connection
	client := pb.NewGreeterClient(server.ClientConn())

	ctx := context.Background()
	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "StartTLS"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "Hello StartTLS" {
		t.Errorf("expected 'Hello StartTLS', got '%s'", resp.Message)
	}
}

func TestServerWithAssertions(t *testing.T) {
	handler := &greeterHandler{
		handler: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
			// Assertion: name must not be empty
			if req.Name == "" {
				return nil, status.Error(codes.InvalidArgument, "name is required")
			}
			// Assertion: name must not be "error"
			if req.Name == "error" {
				return nil, status.Error(codes.Internal, "internal error")
			}

			return &pb.HelloReply{Message: "Hello " + req.Name}, nil
		},
	}

	server := grpctest.NewServer(func(s *grpc.Server) {
		pb.RegisterGreeterServer(s, handler)
	})
	defer server.Close()

	client := pb.NewGreeterClient(server.ClientConn())
	ctx := context.Background()

	t.Run("successful request", func(t *testing.T) {
		resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "World"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Message != "Hello World" {
			t.Errorf("expected 'Hello World', got '%s'", resp.Message)
		}
	})

	t.Run("invalid argument", func(t *testing.T) {
		_, err := client.SayHello(ctx, &pb.HelloRequest{Name: ""})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("internal error", func(t *testing.T) {
		_, err := client.SayHello(ctx, &pb.HelloRequest{Name: "error"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}
		if st.Code() != codes.Internal {
			t.Errorf("expected Internal, got %v", st.Code())
		}
	})
}

func TestMultipleRequests(t *testing.T) {
	callCount := 0
	handler := &greeterHandler{
		handler: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
			callCount++
			return &pb.HelloReply{Message: "Hello " + req.Name}, nil
		},
	}

	server := grpctest.NewServer(func(s *grpc.Server) {
		pb.RegisterGreeterServer(s, handler)
	})
	defer server.Close()

	client := pb.NewGreeterClient(server.ClientConn())
	ctx := context.Background()

	// Make multiple requests
	for i := 0; i < 5; i++ {
		_, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Test"})
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	if callCount != 5 {
		t.Errorf("expected 5 calls, got %d", callCount)
	}
}

func TestServerClose(t *testing.T) {
	handler := &greeterHandler{
		handler: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
			return &pb.HelloReply{Message: "Hello " + req.Name}, nil
		},
	}

	server := grpctest.NewServer(func(s *grpc.Server) {
		pb.RegisterGreeterServer(s, handler)
	})

	// Get client before closing
	client := pb.NewGreeterClient(server.ClientConn())
	ctx := context.Background()

	// First request should work
	_, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Test"})
	if err != nil {
		t.Fatalf("unexpected error before close: %v", err)
	}

	// Close the server
	server.Close()

	// Subsequent requests should fail
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = client.SayHello(ctx, &pb.HelloRequest{Name: "Test"})
	if err == nil {
		t.Error("expected error after server close, got nil")
	}
}

func TestClientCaching(t *testing.T) {
	handler := &greeterHandler{}

	server := grpctest.NewServer(func(s *grpc.Server) {
		pb.RegisterGreeterServer(s, handler)
	})
	defer server.Close()

	// Get client multiple times
	client1 := server.ClientConn()
	client2 := server.ClientConn()

	// Should be the same instance
	if client1 != client2 {
		t.Error("expected ClientConn() to return the same instance")
	}
}
