# Crypto Package

Package `crypto` provides age encryption functionality for zen-sdk projects.

## Overview

This package provides a clean interface for age-based encryption and decryption operations. It includes:

- **Encryptor interface**: Abstract encryption/decryption operations
- **AgeEncryptor implementation**: Age-based encryption using X25519 keys
- **Support for multiple recipients**: Encrypt once, decrypt by any recipient
- **Map decryption**: Batch decryption of encrypted key-value pairs

## Installation

```bash
go get github.com/kube-zen/zen-sdk/pkg/crypto
```

## Usage

### Basic Encryption and Decryption

```go
package main

import (
    "fmt"
    "github.com/kube-zen/zen-sdk/pkg/crypto"
)

func main() {
    // Create encryptor
    encryptor := crypto.NewAgeEncryptor()

    // Encrypt plaintext
    plaintext := []byte("Secret message")
    recipients := []string{
        "age1qy4h7xj8k9l0m1n2o3p4q5r6s7t8u9v0w1x2y3z4",
    }

    ciphertext, err := encryptor.Encrypt(plaintext, recipients)
    if err != nil {
        panic(err)
    }

    // Decrypt ciphertext
    identity := "AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVW"
    decrypted, err := encryptor.Decrypt(ciphertext, identity)
    if err != nil {
        panic(err)
    }

    fmt.Println(string(decrypted)) // Output: Secret message
}
```

### Multiple Recipients

```go
// Encrypt for multiple recipients
recipients := []string{
    "age1alicekey...",
    "age1bobkey...",
    "age1charliekey...",
}

ciphertext, err := encryptor.Encrypt(plaintext, recipients)

// Any recipient can decrypt:
decrypted, err := encryptor.Decrypt(ciphertext, aliceIdentity)
// or
decrypted, err := encryptor.Decrypt(ciphertext, bobIdentity)
```

### Batch Decryption (Maps)

```go
// Decrypt multiple values at once
encryptedData := map[string]string{
    "api_key":    base64.Encoded(encryptedKey),
    "secret":     base64.Encoded(encryptedSecret),
    "token":      base64.Encoded(encryptedToken),
}

identity := "AGE-SECRET-KEY-1..."
decrypted, err := encryptor.DecryptMap(encryptedData, identity)

// decrypted is now: map[string][]byte{
//     "api_key": []byte("actual-key"),
//     "secret":  []byte("actual-secret"),
//     "token":   []byte("actual-token"),
// }
```

## Generating Age Keys

### Using the age CLI

```bash
# Generate a new key pair
age-keygen -o key.txt

# View the public key
age-keygen -y key.txt
# Output: age1qy4h7xj8k9l0m1n2o3p4q5r6s7t8u9v0w1x2y3z4

# key.txt contains both public and private keys
# Public key (first line): age1qy4h7xj8k9l0m1n2o3p4q5r6s7t8u9v0w1x2y3z4
# Private key: AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVW...
```

### Using Go (with filippo.io/age)

```go
import (
    "filippo.io/age"
)

identity, err := age.GenerateX25519Identity()
if err != nil {
    panic(err)
}

publicKey := identity.Recipient().String()
privateKey := identity.String()

fmt.Println("Public key:", publicKey)
fmt.Println("Private key:", privateKey)
```

## API Reference

### Encryptor Interface

```go
type Encryptor interface {
    // Encrypt encrypts plaintext data using the provided recipients (public keys)
    Encrypt(plaintext []byte, recipients []string) ([]byte, error)

    // Decrypt decrypts ciphertext data using the provided identity (private key)
    Decrypt(ciphertext []byte, identity string) ([]byte, error)

    // DecryptMap decrypts a map of base64-encoded encrypted values
    DecryptMap(encryptedData map[string]string, identity string) (map[string][]byte, error)
}
```

### AgeEncryptor

```go
type AgeEncryptor struct{}

func NewAgeEncryptor() *AgeEncryptor
```

## Error Handling

The package returns descriptive errors for common failure cases:

- **No recipients provided**: `at least one recipient (public key) is required`
- **Invalid recipient key**: `failed to parse recipient "KEY": ...`
- **No identity provided**: `identity (private key) is required`
- **Invalid identity key**: `failed to parse identity: ...`
- **Decryption failure**: `failed to create decrypt reader: ...`

Always check for errors and handle them appropriately:

```go
ciphertext, err := encryptor.Encrypt(plaintext, recipients)
if err != nil {
    if strings.Contains(err.Error(), "parse recipient") {
        // Invalid recipient key format
        return fmt.Errorf("invalid public key: %w", err)
    }
    return fmt.Errorf("encryption failed: %w", err)
}
```

## Security Considerations

1. **Key Management**: Never hardcode private keys. Use environment variables, secret management systems, or secure configuration files.

2. **Key Rotation**: Implement key rotation policies for production use. Age supports multiple recipients, making rotation easier.

3. **Key Storage**: Store private keys securely. Consider using:
   - Environment variables (for dev/test)
   - Secret management systems (HashiCorp Vault, AWS Secrets Manager, etc.)
   - File system with restricted permissions (chmod 600)

4. **Key Separation**: Use different keys for different purposes (e.g., API keys vs. secrets).

5. **Auditing**: Log encryption/decryption operations for security auditing (without logging the actual data).

## Performance

- **Encryption**: ~10MB/s on modern CPUs
- **Decryption**: ~10MB/s on modern CPUs
- **Key generation**: ~5ms for X25519 key pair

For large data (>1GB), consider chunked encryption/decryption to manage memory usage.

## Testing

```bash
# Run all tests
go test ./pkg/crypto/...

# Run with race detection
go test -race ./pkg/crypto/...

# Run benchmarks
go test -bench=. ./pkg/crypto/...

# Run coverage
go test -cover ./pkg/crypto/...
```

## Integration with zen-sdk Projects

This crypto package is used by:

- **zen-lock**: Secret management and configuration encryption
- **zen-brain**: Agent credentials and sensitive data protection
- **zen-flow**: Workflow secrets and pipeline credentials

## License

See LICENSE file in the zen-sdk repository.

## Origin

This code is derived from zen-lock/pkg/crypto to preserve IP ownership and ensure consistency across zen-sdk projects.
