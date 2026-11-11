package devtools

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"hash/crc32"
	"strings"
	"time"

	"golang.org/x/crypto/blake2b"
)

// IDKinds lists the ids NewID makes.
var IDKinds = []string{"uuid4", "uuid7", "ulid", "nanoid"}

// NewID makes one id of the kind.
func NewID(kind string, now time.Time) (string, error) {
	switch kind {
	case "uuid4", "uuid":
		var b [16]byte
		rand.Read(b[:])
		b[6] = b[6]&0x0f | 0x40
		b[8] = b[8]&0x3f | 0x80
		return formatUUID(b), nil
	case "uuid7":
		var b [16]byte
		ms := uint64(now.UnixMilli())
		binary.BigEndian.PutUint64(b[:8], ms<<16)
		rand.Read(b[6:])
		b[6] = b[6]&0x0f | 0x70
		b[8] = b[8]&0x3f | 0x80
		return formatUUID(b), nil
	case "ulid":
		return ULID(now), nil
	case "nanoid":
		const alphabet = "_-0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		var b [21]byte
		rand.Read(b[:])
		for i := range b {
			b[i] = alphabet[b[i]&63]
		}
		return string(b[:]), nil
	}
	return "", errors.New("unknown id kind")
}

func formatUUID(b [16]byte) string {
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ULID makes a ULID: 48 bits of milliseconds and 80 random bits in Crockford base32.
func ULID(now time.Time) string {
	var b [16]byte
	ms := uint64(now.UnixMilli())
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	rand.Read(b[6:])
	// 128 bits as 26 characters of 5 bits, the first holding 3.
	var out [26]byte
	var acc uint64
	bits := 0
	pos := 25
	for i := 15; i >= 0; i-- {
		acc |= uint64(b[i]) << bits
		bits += 8
		for bits >= 5 && pos >= 0 {
			out[pos] = crockford[acc&31]
			acc >>= 5
			bits -= 5
			pos--
		}
	}
	if pos >= 0 {
		out[pos] = crockford[acc&31]
	}
	return string(out[:])
}

// IDInfo reads a UUID or ULID: its version and, for time-based ones, when it was made.
type IDInfo struct {
	Kind    string `json:"kind"`
	Version int    `json:"version,omitempty"`
	Variant string `json:"variant,omitempty"`
	Time    string `json:"time,omitempty"`
}

// ReadID decodes a UUID or ULID.
func ReadID(id string) (IDInfo, error) {
	id = strings.TrimSpace(id)
	if len(id) == 26 && strings.Trim(strings.ToUpper(id), crockford) == "" {
		var ms uint64
		for _, c := range strings.ToUpper(id[:10]) {
			ms = ms<<5 | uint64(strings.IndexRune(crockford, c))
		}
		return IDInfo{Kind: "ulid", Time: time.UnixMilli(int64(ms)).UTC().Format(time.RFC3339Nano)}, nil
	}
	raw := strings.ReplaceAll(strings.Trim(id, "{}"), "-", "")
	b, err := hex.DecodeString(raw)
	if err != nil || len(b) != 16 {
		return IDInfo{}, errors.New("not a UUID or ULID")
	}
	info := IDInfo{Kind: "uuid", Version: int(b[6] >> 4)}
	switch {
	case b[8]&0x80 == 0:
		info.Variant = "NCS"
	case b[8]&0xc0 == 0x80:
		info.Variant = "RFC 9562"
	case b[8]&0xe0 == 0xc0:
		info.Variant = "Microsoft"
	default:
		info.Variant = "future"
	}
	switch info.Version {
	case 7:
		ms := binary.BigEndian.Uint64(append([]byte{0, 0}, b[:6]...))
		info.Time = time.UnixMilli(int64(ms)).UTC().Format(time.RFC3339Nano)
	case 1:
		ts := uint64(b[6]&0x0f)<<56 | uint64(b[7])<<48 | uint64(b[4])<<40 | uint64(b[5])<<32 | uint64(binary.BigEndian.Uint32(b[:4]))
		// 100 ns intervals since 15 October 1582.
		unix := int64(ts/10) - 12219292800000000
		info.Time = time.UnixMicro(unix).UTC().Format(time.RFC3339Nano)
	}
	return info, nil
}

// HashKinds lists the digests Hash makes.
var HashKinds = []string{"md5", "sha1", "sha256", "sha384", "sha512", "sha3-256", "sha3-512", "blake2b-256", "crc32"}

func hasher(kind string) (func() hash.Hash, bool) {
	switch kind {
	case "md5":
		return md5.New, true
	case "sha1":
		return sha1.New, true
	case "sha256":
		return sha256.New, true
	case "sha384":
		return sha512.New384, true
	case "sha512":
		return sha512.New, true
	case "sha3-256":
		return func() hash.Hash { return sha3.New256() }, true
	case "sha3-512":
		return func() hash.Hash { return sha3.New512() }, true
	case "blake2b-256":
		return func() hash.Hash { h, _ := blake2b.New256(nil); return h }, true
	case "crc32":
		return func() hash.Hash { return crc32.NewIEEE() }, true
	}
	return nil, false
}

// Digest is one hash of the input, in hex and base64.
type Digest struct {
	Hex    string `json:"hex"`
	Base64 string `json:"base64"`
}

// Hash returns the named digests of data; with a key, HMACs instead.
func Hash(data []byte, kinds []string, key []byte) (map[string]Digest, error) {
	out := map[string]Digest{}
	for _, kind := range kinds {
		make, ok := hasher(kind)
		if !ok {
			return nil, errors.New("unknown hash " + kind + "; use " + strings.Join(HashKinds, ", "))
		}
		var h hash.Hash
		if key != nil {
			if kind == "crc32" {
				return nil, errors.New("crc32 is not a keyed hash")
			}
			h = hmac.New(make, key)
		} else {
			h = make()
		}
		h.Write(data)
		sum := h.Sum(nil)
		out[kind] = Digest{Hex: hex.EncodeToString(sum), Base64: base64.StdEncoding.EncodeToString(sum)}
	}
	return out, nil
}

// JWT is a decoded JSON Web Token. The signature is not checked.
type JWT struct {
	Header    map[string]any `json:"header"`
	Payload   map[string]any `json:"payload"`
	Signature string         `json:"signature"`
	Times     map[string]any `json:"times,omitempty"`
	Expired   *bool          `json:"expired,omitempty"`
	Warnings  []string       `json:"warnings"`
}

// DecodeJWT reads a token without verifying it.
func DecodeJWT(token string, now time.Time) (JWT, error) {
	token = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(token), "Bearer "))
	parts := strings.Split(token, ".")
	if len(parts) == 5 {
		return JWT{}, errors.New("this is an encrypted token (JWE, five parts); it cannot be read without the key")
	}
	if len(parts) != 3 {
		return JWT{}, errors.New("a JWT has three parts separated by dots")
	}
	var t JWT
	t.Warnings = []string{}
	for i, into := range []*map[string]any{&t.Header, &t.Payload} {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[i], "="))
		if err != nil {
			return JWT{}, errors.New([]string{"header", "payload"}[i] + " is not base64url")
		}
		if err := json.Unmarshal(raw, into); err != nil {
			return JWT{}, errors.New([]string{"header", "payload"}[i] + " is not JSON")
		}
	}
	t.Signature = parts[2]
	if alg, _ := t.Header["alg"].(string); strings.EqualFold(alg, "none") || parts[2] == "" {
		t.Warnings = append(t.Warnings, "The token is not signed (alg none): never accept it.")
	}
	t.Times = map[string]any{}
	for _, claim := range []string{"iat", "nbf", "exp"} {
		if v, ok := t.Payload[claim].(float64); ok {
			at := time.Unix(int64(v), 0).UTC()
			t.Times[claim] = at.Format(time.RFC3339)
			if claim == "exp" {
				expired := now.After(at)
				t.Expired = &expired
			}
			if claim == "nbf" && now.Before(at) {
				t.Warnings = append(t.Warnings, "The token is not valid yet (nbf).")
			}
		}
	}
	if len(t.Times) == 0 {
		t.Times = nil
	}
	return t, nil
}
