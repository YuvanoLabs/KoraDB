// Package auth provides API-key authentication and role-based authorization for
// the KoraDB server.
//
// Keys are high-entropy random tokens of the form
//
//	kdb_<keyID hex>_<secret hex>
//
// The "kdb_" prefix lets secret scanners recognize a leaked key. Only the
// SHA-256 of the secret is stored (never the secret itself); because the secret
// is 256 bits of randomness it cannot be brute-forced, so a fast hash plus a
// constant-time comparison is the correct and standard choice (bcrypt/argon2
// exist to slow down guessing of low-entropy passwords, which does not apply
// here). Authorization is a coarse, fail-closed role check: an unmapped method
// is denied.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"KoraDB/internal/storage"
)

// keysBucket stores one KeyRecord per API key, keyed by key id.
var keysBucket = []byte("__auth_keys__")

// ErrInvalidToken is returned for any malformed or unrecognized token. It is
// deliberately generic so authentication failures do not leak which part failed.
var ErrInvalidToken = errors.New("auth: invalid token")

// ErrLastAdmin is returned when revocation would remove the final active
// administrator. Keeping one administrator preserves the running server's
// recovery and key-management path.
var ErrLastAdmin = errors.New("auth: cannot revoke the last admin key")

// Role is a coarse access level; higher roles include lower-role permissions.
type Role int

const (
	RoleNone Role = iota
	RoleReadOnly
	RoleReadWrite
	RoleAdmin
)

func (r Role) String() string {
	switch r {
	case RoleReadOnly:
		return "readonly"
	case RoleReadWrite:
		return "readwrite"
	case RoleAdmin:
		return "admin"
	default:
		return "none"
	}
}

// ParseRole converts a CLI/string role into a Role.
func ParseRole(s string) (Role, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "readonly", "ro":
		return RoleReadOnly, nil
	case "readwrite", "rw":
		return RoleReadWrite, nil
	case "admin":
		return RoleAdmin, nil
	default:
		return RoleNone, fmt.Errorf("auth: unknown role %q (want readonly|readwrite|admin)", s)
	}
}

// Principal is an authenticated caller.
type Principal struct {
	KeyID string
	Name  string
	Role  Role
}

// KeyRecord is the persisted form of an API key (secret stored only as a hash).
type KeyRecord struct {
	KeyID        string `json:"keyId"`
	Name         string `json:"name"`
	Role         Role   `json:"role"`
	SecretSHA256 string `json:"secretSha256"` // hex of sha256(secret)
	CreatedUnix  int64  `json:"createdUnix"`
}

// CreateKey mints a new API key, persists its record, and returns the token.
// The token is the only time the secret is available; only its hash is stored.
func CreateKey(store *storage.Store, name string, role Role) (token, keyID string, err error) {
	if role < RoleReadOnly || role > RoleAdmin {
		return "", "", fmt.Errorf("auth: invalid key role %d", role)
	}
	if err := validatePrincipalName(name); err != nil {
		return "", "", err
	}
	keyIDBytes := make([]byte, 8)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(keyIDBytes); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	keyID = hex.EncodeToString(keyIDBytes)
	secret := hex.EncodeToString(secretBytes)
	token = "kdb_" + keyID + "_" + secret

	rec := KeyRecord{
		KeyID:        keyID,
		Name:         name,
		Role:         role,
		SecretSHA256: hashSecret(secret),
		CreatedUnix:  time.Now().Unix(),
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return "", "", err
	}
	if err := store.Update(func(tx *storage.Txn) error {
		return tx.Put(keysBucket, []byte(keyID), b)
	}); err != nil {
		return "", "", err
	}
	return token, keyID, nil
}

// Authenticate validates a token and returns the corresponding principal.
func Authenticate(store *storage.Store, token string) (*Principal, error) {
	keyID, secret, err := parseToken(token)
	if err != nil {
		return nil, err
	}
	var rec KeyRecord
	found := false
	if err := store.View(func(tx *storage.Txn) error {
		b, err := tx.Get(keysBucket, []byte(keyID))
		if err == storage.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		return json.Unmarshal(b, &rec)
	}); err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrInvalidToken
	}
	// Constant-time comparison of the secret hash.
	if subtle.ConstantTimeCompare([]byte(hashSecret(secret)), []byte(rec.SecretSHA256)) != 1 {
		return nil, ErrInvalidToken
	}
	return &Principal{KeyID: rec.KeyID, Name: rec.Name, Role: rec.Role}, nil
}

// Revoke deletes a key by id. Revoking a missing key is not an error. The last
// administrator cannot be revoked because it would leave no safe management or
// recovery path while the server holds the database file lock.
func Revoke(store *storage.Store, keyID string) error {
	return store.Update(func(tx *storage.Txn) error {
		b, err := tx.Get(keysBucket, []byte(keyID))
		if err == storage.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		var target KeyRecord
		if err := json.Unmarshal(b, &target); err != nil {
			return fmt.Errorf("auth: corrupt key record %q: %w", keyID, err)
		}
		if target.Role == RoleAdmin {
			admins := 0
			if err := tx.Scan(keysBucket, func(_, value []byte) error {
				var rec KeyRecord
				if err := json.Unmarshal(value, &rec); err != nil {
					return fmt.Errorf("auth: corrupt key record: %w", err)
				}
				if rec.Role == RoleAdmin {
					admins++
				}
				return nil
			}); err != nil {
				return err
			}
			if admins <= 1 {
				return ErrLastAdmin
			}
		}
		return tx.Delete(keysBucket, []byte(keyID))
	})
}

// List returns every key record (without secrets).
func List(store *storage.Store) ([]KeyRecord, error) {
	var out []KeyRecord
	err := store.View(func(tx *storage.Txn) error {
		return tx.Scan(keysBucket, func(_, v []byte) error {
			var rec KeyRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return err
			}
			rec.SecretSHA256 = "" // never expose, even the hash
			out = append(out, rec)
			return nil
		})
	})
	return out, err
}

// HasAnyKey reports whether at least one key exists (used to detect whether the
// server has been bootstrapped).
func HasAnyKey(store *storage.Store) (bool, error) {
	any := false
	err := store.View(func(tx *storage.Txn) error {
		return tx.Scan(keysBucket, func(_, _ []byte) error {
			any = true
			return errStopScan
		})
	})
	if err == errStopScan {
		return true, nil
	}
	return any, err
}

// HasAdminKey reports whether at least one administrator key exists. A secured
// server requires an admin key so operators cannot start it in a locked-out
// state with only lower-privileged credentials.
func HasAdminKey(store *storage.Store) (bool, error) {
	found := false
	err := store.View(func(tx *storage.Txn) error {
		return tx.Scan(keysBucket, func(_, value []byte) error {
			var rec KeyRecord
			if err := json.Unmarshal(value, &rec); err != nil {
				return fmt.Errorf("auth: corrupt key record: %w", err)
			}
			if rec.Role == RoleAdmin {
				found = true
				return errStopScan
			}
			return nil
		})
	})
	if err == errStopScan {
		return true, nil
	}
	return found, err
}

var errStopScan = errors.New("stop")

func validatePrincipalName(name string) error {
	if !utf8.ValidString(name) {
		return errors.New("auth: principal name must be valid UTF-8")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("auth: principal name must not be empty")
	}
	if len(name) > 128 {
		return errors.New("auth: principal name exceeds 128 bytes")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return errors.New("auth: principal name must not contain control characters")
		}
	}
	return nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func parseToken(token string) (keyID, secret string, err error) {
	if !strings.HasPrefix(token, "kdb_") {
		return "", "", ErrInvalidToken
	}
	parts := strings.Split(strings.TrimPrefix(token, "kdb_"), "_")
	if len(parts) != 2 || len(parts[0]) != 16 || len(parts[1]) != 64 {
		return "", "", ErrInvalidToken
	}
	// Validate hex so junk never reaches the store lookup.
	if _, err := hex.DecodeString(parts[0]); err != nil {
		return "", "", ErrInvalidToken
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", "", ErrInvalidToken
	}
	return parts[0], parts[1], nil
}

