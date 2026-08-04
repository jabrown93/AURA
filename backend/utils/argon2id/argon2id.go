// Package argon2id verifies passwords against Argon2id hashes in the PHC
// string format produced by the Argon2 reference implementation, e.g.
//
//	$argon2id$v=19$m=65536,t=1,p=2$c29tZXNhbHQ$RdescudvJCsgt3ub+b+dWRWJTmaaJObG
//
// It replaces the (now release-inactive) github.com/alexedwards/argon2id
// wrapper with a hash-format-compatible subset of its API on top of
// golang.org/x/crypto/argon2. AURA never creates hashes, so only decoding and
// verification are implemented.
package argon2id

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	// ErrInvalidHash is returned when the hash isn't a 6-part PHC string.
	ErrInvalidHash = errors.New("argon2id: hash is not in the correct format")

	// ErrIncompatibleVariant is returned for Argon2 variants other than argon2id.
	ErrIncompatibleVariant = errors.New("argon2id: incompatible variant of argon2")

	// ErrIncompatibleVersion is returned when the hash was created with a
	// different Argon2 version than this build of x/crypto implements.
	ErrIncompatibleVersion = errors.New("argon2id: incompatible version of argon2")
)

// Params holds the Argon2id cost parameters and sizes encoded in a hash.
type Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// ComparePasswordAndHash reports whether the plain-text password matches the
// Argon2id hash, deriving a key with the parameters and salt embedded in the
// hash and comparing in constant time.
func ComparePasswordAndHash(password, hash string) (bool, error) {
	params, salt, key, err := DecodeHash(hash)
	if err != nil {
		return false, err
	}

	otherKey := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)

	if subtle.ConstantTimeEq(int32(len(key)), int32(len(otherKey))) == 0 {
		return false, nil
	}
	return subtle.ConstantTimeCompare(key, otherKey) == 1, nil
}

// DecodeHash parses a PHC-format Argon2id hash into its parameters, salt, and
// derived key.
func DecodeHash(hash string) (params *Params, salt, key []byte, err error) {
	vals := strings.Split(hash, "$")
	if len(vals) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	if vals[1] != "argon2id" {
		return nil, nil, nil, ErrIncompatibleVariant
	}

	var version int
	if _, err := fmt.Sscanf(vals[2], "v=%d", &version); err != nil {
		return nil, nil, nil, err
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	params = &Params{}
	if _, err := fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &params.Memory, &params.Iterations, &params.Parallelism); err != nil {
		return nil, nil, nil, err
	}

	salt, err = base64.RawStdEncoding.Strict().DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, err
	}
	params.SaltLength = uint32(len(salt))

	key, err = base64.RawStdEncoding.Strict().DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, err
	}
	params.KeyLength = uint32(len(key))

	return params, salt, key, nil
}
