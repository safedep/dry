package utils

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
)

func TlsConfigFromEnvironment(serverName string) (tls.Config, error) {
	caCert, err := ioutil.ReadFile(os.Getenv("APP_SERVICE_TLS_ROOT_CA"))
	if err != nil {
		return tls.Config{}, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(os.Getenv("APP_SERVICE_TLS_CERT"), os.Getenv("APP_SERVICE_TLS_KEY"))
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
