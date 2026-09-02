package adapters

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
)

func TlsConfigFromEnvironment(serverName string) (tls.Config, error) {
	caCert, err := os.ReadFile(os.Getenv("APP_SERVICE_TLS_ROOT_CA"))
	if err != nil {
		return tls.Config{}, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return tls.Config{}, errors.New("failed to parse a certificate from APP_SERVICE_TLS_ROOT_CA")
	}

	cert, err := tls.LoadX509KeyPair(os.Getenv("APP_SERVICE_TLS_CERT"),
		os.Getenv("APP_SERVICE_TLS_KEY"))
	if err != nil {
		return tls.Config{}, err
	}

	return tls.Config{
		ServerName:   serverName,
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
