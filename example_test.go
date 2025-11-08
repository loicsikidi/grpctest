package grpctest_test

import (
	"context"
	"testing"

	"github.com/loicsikidi/grpctest"
	pb "github.com/loicsikidi/grpctest/proto/hello"
	"google.golang.org/grpc"
)

// simpleGreeter is a minimal implementation that only implements SayHello.
// This demonstrates how to test a single method without implementing all methods.
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

// TestMinimalImplementation demonstrates how to test a single gRPC method
// without implementing all methods of the service interface.
//
// This is useful when you have a service with many methods but only want
// to test one or two of them. The UnimplementedGreeterServer provides
// default implementations for all methods, and we only override the ones
// we care about.
//
// In this example, the Greeter service has 2 methods (SayHello and SayBye),
// but we only implement SayHello. SayBye will use the default implementation
// from UnimplementedGreeterServer.
func TestMinimalImplementation(t *testing.T) {
	server := grpctest.NewServer(func(s *grpc.Server) {
		// Use a simple wrapper struct to implement only the method we want to test
		// Note: Greeter has 2 methods (SayHello, SayBye) but we only implement SayHello
		pb.RegisterGreeterServer(s, &simpleGreeter{
			handleFunc: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
				// Only implement the logic for this one method
				if req.Name == "" {
					return &pb.HelloReply{Message: "Hello, stranger!"}, nil
				}
				return &pb.HelloReply{Message: "Hello, " + req.Name + "!"}, nil
			},
		})
	})
	defer server.Close()

	// Test the method we implemented
	client := pb.NewGreeterClient(server.ClientConn())
	ctx := context.Background()

	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Alice"})
	if err != nil {
		t.Fatalf("SayHello failed: %v", err)
	}

	if resp.Message != "Hello, Alice!" {
		t.Errorf("expected 'Hello, Alice!', got '%s'", resp.Message)
	}

	// Test with empty name
	resp, err = client.SayHello(ctx, &pb.HelloRequest{Name: ""})
	if err != nil {
		t.Fatalf("SayHello with empty name failed: %v", err)
	}

	if resp.Message != "Hello, stranger!" {
		t.Errorf("expected 'Hello, stranger!', got '%s'", resp.Message)
	}
}
