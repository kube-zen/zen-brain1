// Package crypto provides age encryption/decryption utilities for zen-brain1.
// Uses zen-sdk/pkg/crypto under the hood.
package crypto

import (
	"fmt"
	"os"

	"github.com/kube-zen/zen-sdk/pkg/crypto"
)

var (
	// Global encryptor instance
	encryptor crypto.Encryptor
	// Whether encryption is enabled
	enabled bool
	// Initialization status
	initialized bool
)

// Init initializes the crypto package with age keys.
// If AGE_PUBLIC_KEY or AGE_PRIVATE_KEY are not set, encryption is disabled.
func Init() error {
	if initialized {
		return nil
	}

	publicKey := os.Getenv("AGE_PUBLIC_KEY")
	privateKey := os.Getenv("AGE_PRIVATE_KEY")

	// If no keys provided, disable encryption
	if publicKey == "" || privateKey == "" {
		enabled = false
		initialized = true
		return nil
	}

	encryptor = crypto.NewAgeEncryptor()
	enabled = true

	// Test encryption/decryption to verify keys are valid
	testPlaintext := []byte("zen-brain-crypto-test-verification")
	testRecipients := []string{publicKey}
	testCipher, err := encryptor.Encrypt(testPlaintext, testRecipients)
	if err != nil {
		return fmt.Errorf("test encryption failed: %w", err)
	}

	testPlain, err := encryptor.Decrypt(testCipher, privateKey)
	if err != nil {
		return fmt.Errorf("test decryption failed: %w", err)
	}

	if string(testPlain) != string(testPlaintext) {
		return fmt.Errorf("roundtrip verification failed: plaintext mismatch")
	}

	initialized = true
	return nil
}

// Encrypt encrypts plaintext if encryption is enabled.
// If encryption is disabled, returns plaintext unchanged.
func Encrypt(plaintext []byte) ([]byte, error) {
	if !enabled {
		return plaintext, nil
	}

	publicKey := os.Getenv("AGE_PUBLIC_KEY")
	if publicKey == "" {
		return nil, fmt.Errorf("AGE_PUBLIC_KEY not set but encryption enabled")
	}

	recipients := []string{publicKey}
	return encryptor.Encrypt(plaintext, recipients)
}

// Decrypt decrypts ciphertext if encryption is enabled.
// If encryption is disabled, returns ciphertext unchanged.
func Decrypt(ciphertext []byte) ([]byte, error) {
	if !enabled {
		return ciphertext, nil
	}

	privateKey := os.Getenv("AGE_PRIVATE_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("AGE_PRIVATE_KEY not set but encryption enabled")
	}

	return encryptor.Decrypt(ciphertext, privateKey)
}

// IsEnabled returns whether encryption is enabled.
func IsEnabled() bool {
	return enabled
}

// EnsureInitialized ensures crypto is initialized.
// Safe to call multiple times.
func EnsureInitialized() error {
	if !initialized {
		return Init()
	}
	return nil
}
