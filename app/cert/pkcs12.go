package cert

import (
	"crypto"
	"crypto/x509"

	gopkcs12 "software.sslmate.com/src/go-pkcs12"
)

// EncodePKCS12 encodes a private key and certificate into PKCS#12 (P12) format.
func EncodePKCS12(key crypto.PrivateKey, cert *x509.Certificate, password string) ([]byte, error) {
	return gopkcs12.Modern.Encode(key, cert, nil, password)
}
