// Package grpctest provides utilities for testing gRPC servers, similar to httptest for HTTP.
package grpctest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Server represents a gRPC test server, similar to [httptest.Server].
type Server struct {
	// URL is the base URL of the test server (e.g., "localhost:12345").
	// This is set when the server starts.
	URL string

	// TLS is the optional TLS configuration for the server.
	// For servers created with NewTLSServer, this will be populated with
	// a self-signed certificate. For clients to trust the server,
	// use the [Server.Certificate] method to get the server's certificate.
	TLS *tls.Config

	// Listener is the network listener the server is using.
	// It will be set after Start or StartTLS is called.
	Listener net.Listener

	// Config holds optional gRPC server options.
	// You can modify [ServerConfig] before calling Start() or StartTLS()
	// to add interceptors or other gRPC options.
	Config *ServerConfig

	mu      sync.Mutex
	server  *grpc.Server
	started bool
	closed  bool
	client  *grpc.ClientConn
	useTLS  bool
	cert    *x509.Certificate
}

// ServerConfig holds configuration for a test server.
type ServerConfig struct {
	// registerService is called to register gRPC services on the server.
	// This is set during server creation and should not be modified.
	registerService func(*grpc.Server)

	// ServerOptions are optional gRPC server options.
	// These can be modified before calling Start() or StartTLS().
	ServerOptions []grpc.ServerOption
}

// NewServer creates and starts a new gRPC test server listening on a random local port.
// The server runs in plain text mode (non-TLS).
//
// Example:
//
//	server := grpctest.NewServer(func(s *grpc.Server) {
//		proto.RegisterGreeterServer(s, &myGreeterImpl{})
//	})
//	defer server.Close()
//
//	// server.URL contains the address like "localhost:12345"
func NewServer(registerFunc func(*grpc.Server)) *Server {
	s := NewUnstartedServer(registerFunc)
	s.Start()
	return s
}

// NewUnstartedServer creates a new gRPC test server but does not start it.
// The caller must call [Server.Start] or [Server.StartTLS] to start the server.
//
// Example:
//
//	server := grpctest.NewUnstartedServer(func(s *grpc.Server) {
//		proto.RegisterGreeterServer(s, &myGreeterImpl{})
//	})
//	// Configure server as needed
//	server.Start()
//	defer server.Close()
func NewUnstartedServer(registerFunc func(*grpc.Server)) *Server {
	return &Server{
		Config: &ServerConfig{
			registerService: registerFunc,
		},
	}
}

// NewTLSServer creates and starts a new gRPC test server with TLS enabled.
// The server generates a self-signed certificate.
// Clients can use the [Server.Certificate] method to get the certificate for trust configuration.
//
// Example:
//
//	server := grpctest.NewTLSServer(func(s *grpc.Server) {
//		proto.RegisterGreeterServer(s, &myGreeterImpl{})
//	})
//	defer server.Close()
func NewTLSServer(registerFunc func(*grpc.Server)) *Server {
	s := NewUnstartedServer(registerFunc)
	s.StartTLS()
	return s
}

// Start starts the server listening on a random local port in plain text mode.
// If the server is already started, this method does nothing.
//
// Note: this method panics if the server fails to start.
func (s *Server) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}

	s.useTLS = false
	if err := s.start(); err != nil {
		panic(fmt.Sprintf("grpctest: failed to start server: %v", err))
	}
}

// StartTLS starts the server with TLS enabled using a self-signed certificate.
// If the server is already started, this method does nothing.
//
// Note: this method panics if the server fails to start.
func (s *Server) StartTLS() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}

	s.useTLS = true
	if err := s.setupTLS(); err != nil {
		panic(fmt.Sprintf("grpctest: failed to setup TLS: %v", err))
	}
	if err := s.start(); err != nil {
		panic(fmt.Sprintf("grpctest: failed to start server: %v", err))
	}
}

// start is the internal method that actually starts the server.
// Must be called with s.mu held.
func (s *Server) start() error {
	if s.started {
		return nil
	}

	// Create listener on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.Listener = listener
	s.URL = listener.Addr().String()

	// Prepare server options
	opts := s.Config.ServerOptions
	if s.useTLS && s.TLS != nil {
		creds := credentials.NewTLS(s.TLS)
		opts = append(opts, grpc.Creds(creds))
	}

	// Create gRPC server
	s.server = grpc.NewServer(opts...)

	// Register services
	if s.Config.registerService != nil {
		s.Config.registerService(s.server)
	}

	// Start serving in background
	go func() {
		if err := s.server.Serve(s.Listener); err != nil {
			fmt.Printf("grpctest: server error: %v\n", err)
		}
	}()

	s.started = true
	return nil
}

// setupTLS generates a self-signed certificate for the test server.
// Must be called with s.mu held.
func (s *Server) setupTLS() error {
	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"grpctest"},
			CommonName:   "localhost",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	// Create self-signed certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}
	s.cert = cert

	// Encode certificate and key for TLS config
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	// Create TLS certificate
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to create TLS certificate: %w", err)
	}

	// Configure TLS
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS13,
	}

	return nil
}

// Close shuts down the server and releases all resources.
// It's safe to call Close multiple times.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	if s.client != nil {
		s.client.Close() // nolint:errcheck
		s.client = nil
	}

	if s.server != nil {
		s.server.Stop()
		s.server = nil
	}

	if s.Listener != nil {
		s.Listener.Close() // nolint:errcheck
		s.Listener = nil
	}
}

// Certificate returns the server's certificate.
// This is only set for TLS servers created with [NewTLSServer] or servers started with [Server.StartTLS].
// Returns nil if the server is not using TLS.
func (s *Server) Certificate() *x509.Certificate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cert
}

// ClientConn returns a gRPC client connection to the test server.
// For TLS servers, the client is configured to trust the server's self-signed certificate.
//
// Notes:
//   - the connection is cached and reused on subsequent calls.
//   - the connection will be closed when the server is closed.
//   - this method panics if the server is not started.
func (s *Server) ClientConn() grpc.ClientConnInterface {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client
	}

	if !s.started {
		panic("grpctest: server not started")
	}

	var opts []grpc.DialOption

	if s.useTLS {
		// Create cert pool with server's certificate
		certPool := x509.NewCertPool()
		certPool.AddCert(s.cert)

		creds := credentials.NewTLS(&tls.Config{
			RootCAs:    certPool,
			ServerName: "localhost",
		})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(s.URL, opts...)
	if err != nil {
		panic(fmt.Sprintf("grpctest: failed to dial server: %v", err))
	}

	s.client = conn
	return conn
}
