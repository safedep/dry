package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
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

	log.Debugf("Starting %s gRPC server on %s:%s", name, host, port)
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

type tokenCredential struct {
	token                    string
	headers                  http.Header
	requireTransportSecurity bool
}

func (t *tokenCredential) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	h := map[string]string{}
	for k, v := range t.headers {
		if len(v) > 0 && v[0] != "" {
			h[k] = v[0]
		}
	}

	if t.token != "" {
		h["authorization"] = t.token
	}

	return h, nil
}

func (t *tokenCredential) RequireTransportSecurity() bool {
	return t.requireTransportSecurity
}

func GrpcClient(name, host, port string, token string, headers http.Header,
	dopts []grpc.DialOption, configurer ...GrpcClientConfigurer) (*grpc.ClientConn, error) {
	if os.Getenv("INSECURE_GRPC_CLIENT_USE_INSECURE_TRANSPORT") == "true" {
		return GrpcInsecureClient(name, host, port, token, headers, dopts, NoGrpcConfigurer)
	} else {
		return GrpcSecureClient(name, host, port, token, headers, dopts, configurer...)
	}
}

func GrpcInsecureClient(name, host, port string, token string, headers http.Header,
	dopts []grpc.DialOption, configurer GrpcClientConfigurer) (*grpc.ClientConn, error) {
	tc := grpc.WithTransportCredentials(insecure.NewCredentials())
	dopts = append(dopts, tc)
	dopts = append(dopts, grpc.WithPerRPCCredentials(&tokenCredential{
		token:                    token,
		headers:                  headers,
		requireTransportSecurity: false,
	}))

	return grpcClient(name, host, port, dopts, configurer)
}

func GrpcSecureClient(name, host, port string, token string, headers http.Header,
	dopts []grpc.DialOption, configurer ...GrpcClientConfigurer) (*grpc.ClientConn, error) {
	creds := []grpc.DialOption{}
	creds = append(creds, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	creds = append(creds, grpc.WithPerRPCCredentials(&tokenCredential{
		token:                    token,
		headers:                  headers,
		requireTransportSecurity: true,
	}))

	dopts = append(dopts, creds...)
	return grpcClient(name, host, port, dopts, configurer...)
}

func grpcClient(name, host, port string, dopts []grpc.DialOption, configurer ...GrpcClientConfigurer) (*grpc.ClientConn, error) {
	log.Debugf("[%s] Connecting to gRPC server %s:%s", name, host, port)

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
			log.Errorf("[%s] Failed to connect to gRPC server %d/%d : %v",
				name, arg.Current, arg.Total, err)
		}

		return err
	})

	if err != nil {
		return nil, err
	}

	for _, c := range configurer {
		c(conn)
	}

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
