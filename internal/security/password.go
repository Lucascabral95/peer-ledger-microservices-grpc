package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	passwordAlgorithm    = "pbkdf2_sha256"
	defaultSaltLength    = 16
	defaultDerivedKeyLen = 32
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(encodedHash, password string) (bool, error)
}

type PBKDF2Hasher struct {
	Iterations int
	SaltLength int
	KeyLength  int
}

func NewPBKDF2Hasher(iterations int) (*PBKDF2Hasher, error) {
	if iterations <= 0 {
		return nil, errors.New("password iterations must be > 0")
	}

	return &PBKDF2Hasher{
		Iterations: iterations,
		SaltLength: defaultSaltLength,
		KeyLength:  defaultDerivedKeyLen,
	}, nil
}

func (h *PBKDF2Hasher) Hash(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", errors.New("password cannot be empty")
	}

	salt := make([]byte, h.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read random salt: %w", err)
	}

	derivedKey := pbkdf2SHA256([]byte(password), salt, h.Iterations, h.KeyLength)
	return fmt.Sprintf(
		"%s$%d$%s$%s",
		passwordAlgorithm,
		h.Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(derivedKey),
	), nil
}

func (h *PBKDF2Hasher) Compare(encodedHash, password string) (bool, error) {
	if strings.TrimSpace(encodedHash) == "" {
		return false, errors.New("encoded hash cannot be empty")
	}
	if password == "" {
		return false, nil
	}

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 4 {
		return false, errors.New("invalid password hash format")
	}
	if parts[0] != passwordAlgorithm {
		return false, errors.New("unsupported password hash algorithm")
	}

	iterations, err := parsePositiveInt(parts[1])
	if err != nil {
		return false, fmt.Errorf("invalid password hash iterations: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, fmt.Errorf("decode password hash salt: %w", err)
	}

	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, fmt.Errorf("decode password hash digest: %w", err)
	}

	computed := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(expected, computed) == 1, nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	hashLen := sha256.Size
	blockCount := (keyLength + hashLen - 1) / hashLen
	derived := make([]byte, 0, blockCount*hashLen)

	for block := 1; block <= blockCount; block++ {
		u := pbkdf2Block(password, salt, iterations, block)
		derived = append(derived, u...)
	}

	return derived[:keyLength]
}

func pbkdf2Block(password, salt []byte, iterations, blockNum int) []byte {
	var blockBuf [4]byte
	binary.BigEndian.PutUint32(blockBuf[:], uint32(blockNum))

	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write(blockBuf[:])
	u := mac.Sum(nil)

	out := make([]byte, len(u))
	copy(out, u)

	for i := 1; i < iterations; i++ {
		mac = hmac.New(sha256.New, password)
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range out {
			out[j] ^= u[j]
		}
	}

	return out
}

