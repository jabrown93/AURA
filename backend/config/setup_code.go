package config

import (
	"crypto/rand"
	"crypto/subtle"
	"math/big"
	"strings"
	"sync"
)

// The setup code protects setup mode when no login is configured.
// It is printed in the logs when aura starts in setup mode.

// Characters that are easy to read and type, no 0/O or 1/I/L
const setupCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

var setupCode struct {
	mu   sync.Mutex
	code string
}

// SetupCode returns the setup code for this run, like "K7QM-3RXA"
func SetupCode() string {
	setupCode.mu.Lock()
	defer setupCode.mu.Unlock()
	if setupCode.code == "" {
		setupCode.code = generateSetupCode()
	}
	return setupCode.code[:4] + "-" + setupCode.code[4:]
}

// CheckSetupCode reports whether code matches the setup code. Case, spaces and dashes are ignored.
func CheckSetupCode(code string) bool {
	normalized := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	expected := strings.ReplaceAll(SetupCode(), "-", "")
	return len(normalized) == len(expected) && subtle.ConstantTimeCompare([]byte(normalized), []byte(expected)) == 1
}

// generateSetupCode generates a random 8 character setup code
func generateSetupCode() string {
	var builder strings.Builder
	max := big.NewInt(int64(len(setupCodeAlphabet)))
	for range 8 {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic("failed to generate setup code: " + err.Error())
		}
		builder.WriteByte(setupCodeAlphabet[n.Int64()])
	}
	return builder.String()
}
