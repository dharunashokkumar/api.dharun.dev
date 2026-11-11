// Package apikey defines the API key format shared by the accounts program, which makes
// keys, and the API, which checks them.
//
// A key is "india_live_", 32 random base62 characters and a 6-character checksum of
// those characters. The prefix lets secret scanners recognise a leaked key; the checksum
// lets the API refuse a mistyped key without looking it up. Only the SHA-256 hash of a
// key is stored, never the key.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"math/big"
	"strings"
)

// Prefix starts every key.
const Prefix = "india_live_"

const (
	randomLength   = 32
	checksumLength = 6
	alphabet       = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Length is the length of a whole key.
	Length = len(Prefix) + randomLength + checksumLength
)

// ErrFormat is returned for a string that is not a well-formed key.
var ErrFormat = errors.New("not a well-formed India API key")

// New makes a fresh key.
func New() (string, error) {
	var random strings.Builder
	limit := big.NewInt(int64(len(alphabet)))
	for range randomLength {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		random.WriteByte(alphabet[n.Int64()])
	}
	body := random.String()
	return Prefix + body + checksum(body), nil
}

// Check reports whether key is well formed: the prefix, the length, the alphabet and
// the checksum. It does not say whether the key exists.
func Check(key string) error {
	if len(key) != Length || !strings.HasPrefix(key, Prefix) {
		return ErrFormat
	}
	rest := key[len(Prefix):]
	for i := 0; i < len(rest); i++ {
		if strings.IndexByte(alphabet, rest[i]) < 0 {
			return ErrFormat
		}
	}
	body, sum := rest[:randomLength], rest[randomLength:]
	if checksum(body) != sum {
		return ErrFormat
	}
	return nil
}

// Looks reports whether a value is meant as one of our keys (it has the prefix), so a
// caller can tell a mistyped key from some other token a client happens to send.
func Looks(value string) bool { return strings.HasPrefix(value, Prefix) }

// Hash is what is stored for a key: hex SHA-256.
func Hash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Hint is the part of a key that may be shown again after it is made: the prefix and
// the last four characters.
func Hint(key string) string {
	if len(key) < 4 {
		return Prefix + "…"
	}
	return Prefix + "…" + key[len(key)-4:]
}

func checksum(body string) string {
	n := crc32.ChecksumIEEE([]byte(body))
	out := make([]byte, checksumLength)
	for i := checksumLength - 1; i >= 0; i-- {
		out[i] = alphabet[n%uint32(len(alphabet))]
		n /= uint32(len(alphabet))
	}
	return string(out)
}

// TableQuery reads every key with what the API needs to decide on it. The accounts
// program's test runs it against a real accounts database, so the two cannot drift.
// Columns: key id, hash, account id, tier, usable (1 when the key is not revoked, the
// email is verified and the account is not disabled).
const TableQuery = `SELECT k.id, k.hash, k.user, COALESCE(NULLIF(u.tier, ''), 'free'),
	CASE WHEN k.revoked = 0 AND u.verified = 1 AND u.disabled = 0 THEN 1 ELSE 0 END
	FROM api_keys k JOIN users u ON u.id = k.user`

// LookupQuery is TableQuery for one hash.
const LookupQuery = TableQuery + ` WHERE k.hash = ?`
