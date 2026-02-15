package server

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
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// ListenAndServeTLS starts the server with the configured TLS mode.
func (s *Server) ListenAndServeTLS() error {
	switch s.cfg.TLSMode {
	case "autocert":
		return s.listenAutocert()
	case "selfsigned":
		return s.listenSelfSigned()
	case "manual":
		return s.listenManualCert()
	default:
		return s.ListenAndServe()
	}
}

func (s *Server) listenAutocert() error {
	if s.cfg.Domain == "" {
		return fmt.Errorf("domain is required for autocert TLS mode")
	}

	if err := os.MkdirAll(s.cfg.CertCacheDir, 0700); err != nil {
		return fmt.Errorf("create cert cache dir: %w", err)
	}

	m := &autocert.Manager{
		Cache:      autocert.DirCache(s.cfg.CertCacheDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(s.cfg.Domain),
	}

	// Start HTTP challenge server on port 80
	go func() {
		h := m.HTTPHandler(nil)
		s.logger.Info("starting HTTP challenge listener on :80")
		if err := http.ListenAndServe(":80", h); err != nil {
			s.logger.Error("HTTP challenge listener error", "err", err)
		}
	}()

	srv := &http.Server{
		Addr:      s.cfg.ListenAddr,
		Handler:   s.router,
		TLSConfig: m.TLSConfig(),
	}

	s.logger.Info("starting HTTPS server (autocert)",
		"addr", s.cfg.ListenAddr,
		"domain", s.cfg.Domain)

	return srv.ListenAndServeTLS("", "")
}

func (s *Server) listenSelfSigned() error {
	certFile, keyFile, err := s.ensureSelfSignedCert()
	if err != nil {
		return fmt.Errorf("self-signed cert: %w", err)
	}

	s.logger.Info("starting HTTPS server (self-signed)",
		"addr", s.cfg.ListenAddr,
		"cert", certFile)

	return http.ListenAndServeTLS(s.cfg.ListenAddr, certFile, keyFile, s.router)
}

func (s *Server) listenManualCert() error {
	if s.cfg.CertFile == "" || s.cfg.KeyFile == "" {
		return fmt.Errorf("cert_file and key_file are required for manual TLS mode")
	}

	s.logger.Info("starting HTTPS server (manual cert)",
		"addr", s.cfg.ListenAddr,
		"cert", s.cfg.CertFile)

	return http.ListenAndServeTLS(s.cfg.ListenAddr, s.cfg.CertFile, s.cfg.KeyFile, s.router)
}

// ensureSelfSignedCert generates a self-signed cert if one doesn't already exist.
func (s *Server) ensureSelfSignedCert() (certFile, keyFile string, err error) {
	if err := os.MkdirAll(s.cfg.CertCacheDir, 0700); err != nil {
		return "", "", fmt.Errorf("create cert dir: %w", err)
	}

	certFile = filepath.Join(s.cfg.CertCacheDir, "selfsigned.crt")
	keyFile = filepath.Join(s.cfg.CertCacheDir, "selfsigned.key")

	// Check if certs already exist
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			// Verify they're still valid (not expired)
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err == nil {
				leaf, err := x509.ParseCertificate(cert.Certificate[0])
				if err == nil && leaf.NotAfter.After(time.Now().Add(24*time.Hour)) {
					return certFile, keyFile, nil
				}
			}
			// Invalid or expiring, regenerate
		}
	}

	s.logger.Info("generating self-signed certificate")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"MachineMon"},
			CommonName:   "MachineMon Server",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}

	// Write cert
	certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", "", fmt.Errorf("write cert: %w", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	// Write key
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", fmt.Errorf("marshal key: %w", err)
	}
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("write key: %w", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	s.logger.Info("self-signed certificate generated",
		"cert", certFile,
		"expires", template.NotAfter.Format("2006-01-02"))

	return certFile, keyFile, nil
}
