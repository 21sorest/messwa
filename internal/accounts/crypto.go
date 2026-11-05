package accounts

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id settings. Tests lower them.
var (
	argonTime   uint32 = 3
	argonMemory uint32 = 64 * 1024 // KiB
	argonLanes  uint8  = 2
)

const argonKeyLen = 32

// CheapHashing makes password hashing fast and weak. Only tests call it.
func CheapHashing() { argonTime, argonMemory, argonLanes = 1, 8, 1 }

// hashPassword returns a PHC-style argon2id string.
func hashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonLanes, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonLanes, b64.EncodeToString(salt), b64.EncodeToString(sum)), nil
}

// checkPassword reports whether pw matches a hash from hashPassword. The
// hash's own parameters are used, so changing the defaults keeps old
// passwords working.
func checkPassword(hash, pw string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || p == 0 || t == 0 {
		return false
	}
	b64 := base64.RawStdEncoding
	salt, err1 := b64.DecodeString(parts[4])
	want, err2 := b64.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// Box seals mess API keys with AES-256-GCM. Each sealed key is bound to its
// user ID, so copying the column to another row doesn't decrypt.
type Box struct{ aead cipher.AEAD }

// NewBox takes 32 bytes as 64 hex characters.
func NewBox(hexKey string) (*Box, error) {
	key, err := hex.DecodeString(strings.TrimSpace(hexKey))
	if err != nil || len(key) != 32 {
		return nil, errors.New(SecretEnv + " must be 64 hex characters (openssl rand -hex 32)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead}, nil
}

const sealVersion = 1

func sealAD(userID int64) []byte {
	return []byte("messwa mess key v1 user " + strconv.FormatInt(userID, 10))
}

func (b *Box) Seal(userID int64, plain []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize(), 1+b.aead.NonceSize()+len(plain)+b.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{sealVersion}, nonce...)
	return b.aead.Seal(out, nonce, plain, sealAD(userID)), nil
}

func (b *Box) Open(userID int64, sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < 1+n || sealed[0] != sealVersion {
		return nil, errors.New("stored mess API key is unreadable")
	}
	plain, err := b.aead.Open(nil, sealed[1:1+n], sealed[1+n:], sealAD(userID))
	if err != nil {
		return nil, errors.New("stored mess API key can't be decrypted; was " + SecretEnv + " changed?")
	}
	return plain, nil
}

// newToken returns a random URL-safe token and its SHA-256.
func newToken(prefix string) (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	tok := prefix + base64.RawURLEncoding.EncodeToString(b)
	return tok, hashToken(tok), nil
}

func hashToken(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}
