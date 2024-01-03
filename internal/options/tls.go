package options

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

type TLSOptions struct {
	Enabled bool
	Ca      string
	Cert    string
	Key     string
}

func loadTLSCerts(tlsOptions TLSOptions) (*x509.CertPool, []tls.Certificate, error) {
	// Load certificate of the CA who signed console/server's certificate
	pemCA, err := os.ReadFile(tlsOptions.Ca)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open CA cert '%s':\n\t%w", tlsOptions.Ca, err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemCA) {
		return nil, nil, fmt.Errorf("failed to add CA's certificate to cert pool")
	}

	// Load console/server's certificate and private key
	cert, err := tls.LoadX509KeyPair(tlsOptions.Cert, tlsOptions.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open cert/key '%s / %s':\n\t%w",
			tlsOptions.Cert,
			tlsOptions.Key, err)
	}

	return certPool, []tls.Certificate{cert}, nil
}
