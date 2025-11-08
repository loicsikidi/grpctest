package grpctest

import (
	"context"
	"fmt"

	pb "github.com/loicsikidi/grpctest/proto/hello"
)

// GreeterServer is a helper implementation of [pb.GreeterServer] designed for testing.
// It provides optional handler functions that can be set to customize behavior,
// making it particularly useful for testing interceptors or custom handlers
// without writing boilerplate service implementations.
//
// Example usage for testing an interceptor:
//
//	server := grpctest.NewServer(func(s *grpc.Server) {
//		greeter := &grpctest.GreeterServer{
//			SayHelloHandler: func(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
//				return &pb.HelloReply{Message: "Hello " + req.Name}, nil
//			},
//		}
//		pb.RegisterGreeterServer(s, greeter)
//	})
type GreeterServer struct {
	pb.UnimplementedGreeterServer

	// SayHelloHandler is an optional handler for the SayHello RPC.
	// If nil, returns a default response: "Hello <name>".
	SayHelloHandler func(context.Context, *pb.HelloRequest) (*pb.HelloReply, error)

	// SayHelloStreamHandler is an optional handler for the SayHelloStream bidirectional streaming RPC.
	// If nil, uses the default behavior: reads the first message from client,
	// sends back "hello <name>, I'm sorry I'm busy..., bye", and closes the stream.
	SayHelloStreamHandler func(pb.Greeter_SayHelloStreamServer) error
}

// SayHello implements [pb.GreeterServer.SayHello].
// If [GreeterServer.SayHelloHandler] is set, it delegates to that handler.
// Otherwise, returns a default response with message "Hello <name>".
func (g *GreeterServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	if g.SayHelloHandler != nil {
		return g.SayHelloHandler(ctx, req)
	}
	return &pb.HelloReply{Message: "Hello " + req.Name}, nil
}

// SayHelloStream implements [pb.GreeterServer.SayHelloStream].
// If [GreeterServer.SayHelloStreamHandler] is set, it delegates to that handler.
// Otherwise, uses default behavior: receives the first message from the client,
// sends back "hello <name>, I'm sorry I'm busy..., bye", and closes the stream.
func (g *GreeterServer) SayHelloStream(stream pb.Greeter_SayHelloStreamServer) error {
	if g.SayHelloStreamHandler != nil {
		return g.SayHelloStreamHandler(stream)
	}

	// Default implementation: receive first message
	req, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive: %w", err)
	}

	// Send response and close
	if err := stream.Send(&pb.HelloReply{
		Message: fmt.Sprintf("hello %s, I'm sorry I'm busy......, bye", req.Name),
	}); err != nil {
		return fmt.Errorf("failed to send: %w", err)
	}

	return nil
}
