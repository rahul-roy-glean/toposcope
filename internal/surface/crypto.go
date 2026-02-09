package surface

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"strings"
)

// base64URLEncode encodes data using unpadded base64url encoding (RFC 7515).
func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// rsaSign signs the data using RS256 (RSASSA-PKCS1-v1_5 with SHA-256).
func rsaSign(data []byte, key *rsa.PrivateKey) ([]byte, error) {
	h := crypto.SHA256.New()
	h.Write(data)
	return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h.Sum(nil))
}
