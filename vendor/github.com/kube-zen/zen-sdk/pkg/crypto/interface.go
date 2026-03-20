package crypto

// Encryptor defines the interface for encryption/decryption operations
type Encryptor interface {
	// Encrypt encrypts plaintext data using the provided recipients (public keys)
	Encrypt(plaintext []byte, recipients []string) ([]byte, error)

	// Decrypt decrypts ciphertext data using the provided identity (private key)
	Decrypt(ciphertext []byte, identity string) ([]byte, error)

	// DecryptMap decrypts a map of base64-encoded encrypted values
	DecryptMap(encryptedData map[string]string, identity string) (map[string][]byte, error)
}
