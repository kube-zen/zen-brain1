# Crypto Integration Plan

## Target Components

Crypto (Age encryption) should be integrated into zen-brain1 components that handle sensitive data:

### 1. Internal Config (`internal/config/`)
- Encrypt sensitive configuration values at rest
- Decrypt on startup

### 2. Session Management (`internal/session/`)
- Encrypt session tokens
- Encrypt sensitive session metadata

### 3. Office Connectors (`internal/office/`)
- Encrypt API credentials (Jira tokens, etc.)
- Encrypt webhook secrets

### 4. LLM Gateway (`internal/llm/`)
- Encrypt API keys for external LLM providers
- Encrypt authentication tokens

---

## Implementation Pattern

### Step 1: Create Crypto Helper

Create `internal/crypto/crypto.go`:

```go
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
)

// Init initializes the crypto package with age keys
func Init() error {
	publicKey := os.Getenv("AGE_PUBLIC_KEY")
	privateKey := os.Getenv("AGE_PRIVATE_KEY")

	// If no keys provided, disable encryption
	if publicKey == "" || privateKey == "" {
		enabled = false
		return nil
	}

	encryptor = crypto.NewAgeEncryptor()
	enabled = true

	// Test encryption/decryption
	testPlaintext := []byte("test")
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
		return fmt.Errorf("roundtrip verification failed")
	}

	return nil
}

// Encrypt encrypts plaintext if encryption is enabled
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

// Decrypt decrypts ciphertext if encryption is enabled
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

// IsEnabled returns whether encryption is enabled
func IsEnabled() bool {
	return enabled
}
```

---

### Step 2: Integrate into Config

Modify `internal/config/config.go` to encrypt sensitive values:

```go
// Add encrypted fields to Config struct
type Config struct {
	// ... existing fields ...

	// Encrypted secrets (stored encrypted)
	EncryptedAPIToken      string `json:"encrypted_api_token,omitempty"`
	EncryptedWebhookSecret string `json:"encrypted_webhook_secret,omitempty"`

	// Decrypted secrets (in-memory only, not persisted)
	APIToken      string `json:"-"`
	WebhookSecret string `json:"-"`
}

// DecryptSecrets decrypts encrypted secrets in the config
func (c *Config) DecryptSecrets() error {
	if !zencrypto.IsEnabled() {
		c.APIToken = c.EncryptedAPIToken
		c.WebhookSecret = c.EncryptedWebhookSecret
		return nil
	}

	if c.EncryptedAPIToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(c.EncryptedAPIToken)
		if err != nil {
			return fmt.Errorf("failed to decode api token: %w", err)
		}
		decrypted, err := zencrypto.Decrypt(decoded)
		if err != nil {
			return fmt.Errorf("failed to decrypt api token: %w", err)
		}
		c.APIToken = string(decrypted)
	}

	// Similar for other secrets...

	return nil
}

// EncryptSecrets encrypts secrets in the config
func (c *Config) EncryptSecrets() error {
	if !zencrypto.IsEnabled() {
		c.EncryptedAPIToken = c.APIToken
		c.EncryptedWebhookSecret = c.WebhookSecret
		return nil
	}

	if c.APIToken != "" {
		encrypted, err := zencrypto.Encrypt([]byte(c.APITToken))
		if err != nil {
			return fmt.Errorf("failed to encrypt api token: %w", err)
		}
		c.EncryptedAPIToken = base64.StdEncoding.EncodeToString(encrypted)
		c.APIToken = "" // Clear in-memory secret
	}

	// Similar for other secrets...

	return nil
}
```

---

### Step 3: Integrate into Session Store

Modify `internal/session/sqlite_store.go`:

```go
// Add encrypted fields to Session struct
type Session struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Encrypted sensitive data
	EncryptedData string `json:"encrypted_data,omitempty"`

	// Decrypted data (in-memory only)
	Data map[string]interface{} `json:"-"`
}

// Save encrypts sensitive data before persisting
func (s *Session) Save(ctx context.Context) error {
	// Encrypt data before saving
	if zencrypto.IsEnabled() && len(s.Data) > 0 {
		dataJSON, err := json.Marshal(s.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal session data: %w", err)
		}
		encrypted, err := zencrypto.Encrypt(dataJSON)
		if err != nil {
			return fmt.Errorf("failed to encrypt session data: %w", err)
		}
		s.EncryptedData = base64.StdEncoding.EncodeToString(encrypted)
	}

	// Save to database...
}

// Load decrypts sensitive data after loading
func (s *Session) Load(ctx context.Context) error {
	// Load from database...

	// Decrypt data after loading
	if s.EncryptedData != "" && zencrypto.IsEnabled() {
		decoded, err := base64.StdEncoding.DecodeString(s.EncryptedData)
		if err != nil {
			return fmt.Errorf("failed to decode session data: %w", err)
		}
		decrypted, err := zencrypto.Decrypt(decoded)
		if err != nil {
			return fmt.Errorf("failed to decrypt session data: %w", err)
		}
		s.Data = make(map[string]interface{})
		if err := json.Unmarshal(decrypted, &s.Data); err != nil {
			return fmt.Errorf("failed to unmarshal session data: %w", err)
		}
	}

	return nil
}
```

---

### Step 4: Integrate into Office Connectors

Modify `internal/office/jira/connector.go`:

```go
type ConnectorConfig struct {
	URL    string
	// Encrypted credentials
	EncryptedToken string `json:"encrypted_token,omitempty"`
	Token         string `json:"-"` // In-memory only
}

// NewConnector creates a new Jira connector with decrypted credentials
func NewConnector(cfg *ConnectorConfig) (*Connector, error) {
	if zencrypto.IsEnabled() && cfg.EncryptedToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(cfg.EncryptedToken)
		if err != nil {
			return nil, fmt.Errorf("failed to decode token: %w", err)
		}
		decrypted, err := zencrypto.Decrypt(decoded)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt token: %w", err)
		}
		cfg.Token = string(decrypted)
	}

	return &Connector{cfg: cfg}, nil
}

// EncryptCredentials encrypts connector credentials
func (c *Connector) EncryptCredentials() error {
	if !zencrypto.IsEnabled() {
		c.cfg.EncryptedToken = c.cfg.Token
		c.cfg.Token = ""
		return nil
	}

	if c.cfg.Token != "" {
		encrypted, err := zencrypto.Encrypt([]byte(c.cfg.Token))
		if err != nil {
			return fmt.Errorf("failed to encrypt token: %w", err)
		}
		c.cfg.EncryptedToken = base64.StdEncoding.EncodeToString(encrypted)
		c.cfg.Token = "" // Clear in-memory secret
	}

	return nil
}
```

---

### Step 5: Generate Age Keys

Create setup script `scripts/generate-age-keys.sh`:

```bash
#!/bin/bash
set -e

AGE_KEY_FILE="${ZEN_BRAIN_HOME:-$HOME/.zen-brain}/age-key.txt"

echo "Generating age keys..."

# Generate key pair
age-keygen -o "$AGE_KEY_FILE"

echo "✓ Age keys generated at: $AGE_KEY_FILE"
echo ""
echo "Public key (AGE_PUBLIC_KEY):"
age-keygen -y "$AGE_KEY_FILE"
echo ""
echo "Private key (AGE_PRIVATE_KEY) is in: $AGE_KEY_FILE"
echo ""
echo "Add these to your environment:"
echo "export AGE_PUBLIC_KEY=\"$(age-keygen -y "$AGE_KEY_FILE")\""
echo "export AGE_PRIVATE_KEY=\"\$(cat $AGE_KEY_FILE)\""
```

---

### Step 6: Documentation

Create `docs/03-DESIGN/CRYPTO_INTEGRATION.md`:

```markdown
# Crypto (Age Encryption) Integration

## Overview

zen-brain1 uses age encryption (via zen-sdk/pkg/crypto) to secure sensitive data at rest:
- Configuration secrets (API tokens, webhook secrets)
- Session data
- Connector credentials
- LLM provider API keys

## Setup

### 1. Generate Age Keys

```bash
./scripts/generate-age-keys.sh
```

Or manually:
```bash
age-keygen -o ~/.zen-brain/age-key.txt
age-keygen -y ~/.zen-brain/age-key.txt  # Shows public key
```

### 2. Set Environment Variables

```bash
export AGE_PUBLIC_KEY="age1qy4h7xj8k9l0m1n2o3p4q5r6s7t8u9v0w1x2y3z4"
export AGE_PRIVATE_KEY="AGE-SECRET-KEY-1..."
```

### 3. Initialize Crypto

Crypto is initialized automatically on startup if keys are present.

## Usage

### Encryption is Transparent

Once configured, encryption/decryption is automatic:

```go
// Config automatically encrypts secrets when saving
cfg.APIToken = "secret"
cfg.EncryptSecrets()
SaveConfig(cfg) // Encrypted

// Config automatically decrypts secrets on load
cfg = LoadConfig()
cfg.DecryptSecrets() // Decrypted
```

### Multiple Recipients

Age supports encrypting for multiple recipients:

```go
recipients := []string{
    os.Getenv("AGE_PUBLIC_KEY"),
    "age1backupkey...",
}
encryptor.Encrypt(plaintext, recipients)
```

## Security Best Practices

1. **Never commit private keys** - Add `age-key.txt` to `.gitignore`
2. **Rotate keys periodically** - Use multiple recipients for smooth rotation
3. **Use environment variables** - Don't hardcode keys
4. **Encrypt at rest only** - Secrets are decrypted in-memory only
5. **No logging of secrets** - Never log decrypted values

## Troubleshooting

### Encryption Disabled

If `AGE_PUBLIC_KEY` or `AGE_PRIVATE_KEY` are not set, encryption is disabled (secrets stored plaintext).

### Decryption Fails

- Verify keys match (public/private pair)
- Check environment variables are set
- Ensure age-key.txt is readable (chmod 600)

## References

- [zen-sdk/pkg/crypto](../../../zen-sdk/pkg/crypto/README.md)
- [age documentation](https://age-encryption.org/)
```

---

## Migration Steps

### 1. Generate Keys
```bash
./scripts/generate-age-keys.sh
```

### 2. Add to Environment
```bash
# Add to ~/.bashrc or ~/.zshrc
export AGE_PUBLIC_KEY="age1..."
export AGE_PRIVATE_KEY="AGE-SECRET-KEY-1..."
```

### 3. Migrate Existing Secrets

```bash
# Encrypt existing config
zen-brain encrypt-config --config ~/.zen-brain/config.yaml
```

### 4. Test Decryption

```bash
# Verify decryption works
zen-brain decrypt-config --config ~/.zen-brain/config.yaml
```

---

## Testing

```bash
# Test crypto package
go test ./internal/crypto/...

# Test config encryption
go test ./internal/config/...

# Test session encryption
go test ./internal/session/...
```

---

## Success Criteria

- ✅ Age keys generation script
- ✅ Crypto helper package created
- ✅ Config secrets encrypted at rest
- ✅ Session data encrypted
- ✅ Office connector credentials encrypted
- ✅ Documentation complete
- ✅ Tests passing
- ✅ No performance regression (encryption overhead <1ms)
