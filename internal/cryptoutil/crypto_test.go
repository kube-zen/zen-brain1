package cryptoutil

import (
	"os"
	"testing"

	"filippo.io/age"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_NoKeys(t *testing.T) {
	// Save original env values
	oldPublic := os.Getenv("AGE_PUBLIC_KEY")
	oldPrivate := os.Getenv("AGE_PRIVATE_KEY")
	defer func() {
		os.Setenv("AGE_PUBLIC_KEY", oldPublic)
		os.Setenv("AGE_PRIVATE_KEY", oldPrivate)
	}()

	// Clear keys
	os.Unsetenv("AGE_PUBLIC_KEY")
	os.Unsetenv("AGE_PRIVATE_KEY")

	// Reset initialization state
	initialized = false

	// Init should succeed but encryption disabled
	err := Init()
	assert.NoError(t, err)
	assert.False(t, IsEnabled())
}

func TestInit_WithValidKeys(t *testing.T) {
	// Generate temporary age keys
	identity, err := generateTestIdentity()
	require.NoError(t, err)
	defer cleanupTestIdentity(identity)

	// Set environment variables
	os.Setenv("AGE_PUBLIC_KEY", identity.Recipient().String())
	os.Setenv("AGE_PRIVATE_KEY", identity.String())
	defer func() {
		os.Unsetenv("AGE_PUBLIC_KEY")
		os.Unsetenv("AGE_PRIVATE_KEY")
	}()

	// Reset initialization state
	initialized = false

	// Init should succeed with encryption enabled
	err = Init()
	assert.NoError(t, err)
	assert.True(t, IsEnabled())
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	// Setup
	identity, err := generateTestIdentity()
	require.NoError(t, err)
	defer cleanupTestIdentity(identity)

	os.Setenv("AGE_PUBLIC_KEY", identity.Recipient().String())
	os.Setenv("AGE_PRIVATE_KEY", identity.String())
	defer func() {
		os.Unsetenv("AGE_PUBLIC_KEY")
		os.Unsetenv("AGE_PRIVATE_KEY")
	}()

	initialized = false
	err = Init()
	require.NoError(t, err)

	// Test
	plaintext := []byte("test secret message")
	ciphertext, err := Encrypt(plaintext)
	assert.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := Decrypt(ciphertext)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_Disabled(t *testing.T) {
	// Clear keys to disable encryption
	os.Unsetenv("AGE_PUBLIC_KEY")
	os.Unsetenv("AGE_PRIVATE_KEY")
	defer func() {
		os.Setenv("AGE_PUBLIC_KEY", os.Getenv("AGE_PUBLIC_KEY"))
		os.Setenv("AGE_PRIVATE_KEY", os.Getenv("AGE_PRIVATE_KEY"))
	}()

	initialized = false
	err := Init()
	require.NoError(t, err)

	// Encrypt should return plaintext unchanged
	plaintext := []byte("test message")
	ciphertext, err := Encrypt(plaintext)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, ciphertext)
}

func TestDecrypt_Disabled(t *testing.T) {
	// Clear keys to disable encryption
	os.Unsetenv("AGE_PUBLIC_KEY")
	os.Unsetenv("AGE_PRIVATE_KEY")
	defer func() {
		os.Setenv("AGE_PUBLIC_KEY", os.Getenv("AGE_PUBLIC_KEY"))
		os.Setenv("AGE_PRIVATE_KEY", os.Getenv("AGE_PRIVATE_KEY"))
	}()

	initialized = false
	err := Init()
	require.NoError(t, err)

	// Decrypt should return ciphertext unchanged
	ciphertext := []byte("test message")
	plaintext, err := Decrypt(ciphertext)
	assert.NoError(t, err)
	assert.Equal(t, ciphertext, plaintext)
}

func TestEnsureInitialized(t *testing.T) {
	// Reset
	initialized = false
	os.Unsetenv("AGE_PUBLIC_KEY")
	os.Unsetenv("AGE_PRIVATE_KEY")

	// First call should initialize
	err := EnsureInitialized()
	assert.NoError(t, err)
	assert.True(t, initialized)

	// Second call should not re-initialize
	err = EnsureInitialized()
	assert.NoError(t, err)
	assert.True(t, initialized)
}

// Helper functions for testing

func generateTestIdentity() (*age.X25519Identity, error) {
	return age.GenerateX25519Identity()
}

func cleanupTestIdentity(identity *age.X25519Identity) {
	// Nothing to clean up for in-memory identities
}
