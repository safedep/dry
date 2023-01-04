package adapters

import (
	"fmt"
	"net"
	"time"

	"github.com/safedep/dry/adapters"
	"github.com/safedep/dry/log"
	"github.com/safedep/dry/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpcotel "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

type GrpcAdapterConfigurer func(server *grpc.Server)
type GrpcClientConfigurer func(conn *grpc.ClientConn)

var (
	NoGrpcDialOptions = []grpc.DialOption{}
	NoGrpcConfigurer  = func(conn *grpc.ClientConn) {}
)

func StartGrpcMtlsServer(name, serverName, host, port string,
	sopts []grpc.ServerOption, configure GrpcAdapterConfigurer) {
	tc, err := adapters.TlsConfigFromEnvironment(serverName)
	if err != nil {
		log.Fatalf("Failed to setup TLS from environment: %v", err)
	}

	creds := credentials.NewTLS(&tc)
	sopts = append(sopts, grpc.Creds(creds))

	StartGrpcServer(name, host, port, sopts, configure)
}

func StartGrpcServer(name, host, port string, sopts []grpc.ServerOption,
	configure GrpcAdapterConfigurer) {
	addr := net.JoinHostPort(host, port)
	listener, err := net.Listen("tcp", addr)

	if err != nil {
		log.Fatalf("Failed to listen on %s:%s - %s", host, port, err.Error())
	}

	sopts = append(sopts, grpc.UnaryInterceptor(
		grpc_middleware.ChainUnaryServer(
			grpcotel.UnaryServerInterceptor(),
			grpc_validator.UnaryServerInterceptor(),
		),
	))

	sopts = append(sopts, grpc.StreamInterceptor(
		grpc_middleware.ChainStreamServer(
			grpcotel.StreamServerInterceptor(),
			grpc_validator.StreamServerInterceptor(),
		),
	))

	server := grpc.NewServer(sopts...)
	configure(server)

	log.Infof("Starting %s gRPC server on %s:%s", name, host, port)
	err = server.Serve(listener)

	log.Errorf("gRPC Server exit: %s", err.Error())
}

func GrpcMtlsClient(name, serverName, host, port string, dopts []grpc.DialOption,
	configurer GrpcClientConfigurer) (*grpc.ClientConn, error) {
	tc, err := grpcTransportCredentials(serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to setup client transport credentials: %w", err)
	}

	dopts = append(dopts, tc)
	return grpcClient(name, host, port, dopts, configurer)
}

func GrpcInsecureClient(name, host, port string, dopts []grpc.DialOption, configurer GrpcClientConfigurer) (*grpc.ClientConn, error) {
	tc := grpc.WithTransportCredentials(insecure.NewCredentials())
	dopts = append(dopts, tc)
	return grpcClient(name, host, port, dopts, configurer)
}

func grpcClient(name, host, port string, dopts []grpc.DialOption, configurer GrpcClientConfigurer) (*grpc.ClientConn, error) {
	log.Infof("[%s] Connecting to gRPC server %s:%s", name, host, port)

	dopts = append(dopts, grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()))
	dopts = append(dopts, grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))

	var conn *grpc.ClientConn
	var err error

	retry.InvokeWithRetry(retry.RetryConfig{
		Count: 10,
		Sleep: 1 * time.Second,
	}, func(arg retry.RetryFuncArg) error {
		conn, err = grpc.Dial(net.JoinHostPort(host, port), dopts...)
		if err != nil {
			log.Infof("[%s] Failed to connect to gRPC server %d/%d : %v",
				name, arg.Current, arg.Total, err)
		}

		return err
	})

	if err != nil {
		return nil, err
	}

	configurer(conn)
	return conn, nil
}

func grpcTransportCredentials(serverName string) (grpc.DialOption, error) {
	tlsConfig, err := adapters.TlsConfigFromEnvironment(serverName)
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(&tlsConfig)
	return grpc.WithTransportCredentials(creds), nil
}
