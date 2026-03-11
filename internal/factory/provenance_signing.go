package factory

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SigningKeyType defines the type of key used for signing
type SigningKeyType string

const (
	// SigningKeyNone means no signing (fallback)
	SigningKeyNone SigningKeyType = "none"
	// SigningKeyRSA means RSA private key signing
	SigningKeyRSA SigningKeyType = "rsa"
	// SigningKeyEd25519 means Ed25519 signing
	SigningKeyEd25519 SigningKeyType = "ed25519"
)

// SignatureProof represents a cryptographic signature for proof-of-work
type SignatureProof struct {
	Signature     string    `json:"signature"`
	KeyID         string    `json:"key_id"`
	SigningMethod string    `json:"signing_method"`
	Algorithm     string    `json:"algorithm"`
	Timestamp     time.Time `json:"timestamp"`
	DataHash      string    `json:"data_hash"`
}

// SigningConfig configures how proofs are signed
type SigningConfig struct {
	Enabled          bool          `json:"enabled"`
	KeyType          SigningKeyType `json:"key_type"`
	PrivateKeyPath   string        `json:"private_key_path"`
	PublicKeyPath    string        `json:"public_key_path"`
	KeyID            string        `json:"key_id"`
	FallbackBehavior string        `json:"fallback_behavior"` // "fail", "warn", "ignore"
}

// DefaultSigningConfig returns a default signing configuration
func DefaultSigningConfig() SigningConfig {
	return SigningConfig{
		Enabled:          false,
		KeyType:          SigningKeyRSA,
		PrivateKeyPath:   "",
		PublicKeyPath:    "",
		KeyID:            "",
		FallbackBehavior: "warn",
	}
}

// ProofSigner handles cryptographic signing of proof artifacts
type ProofSigner struct {
	config SigningConfig
}

// NewProofSigner creates a new proof signer with the given configuration
func NewProofSigner(config SigningConfig) *ProofSigner {
	return &ProofSigner{
		config: config,
	}
}

// SignProof signs the proof-of-work data and returns a signature proof
func (ps *ProofSigner) SignProof(proofData string) (*SignatureProof, error) {
	// Compute hash of the proof data
	hash := sha256.Sum256([]byte(proofData))
	dataHash := hex.EncodeToString(hash[:])

	// If signing is not enabled, return unsigned proof
	if !ps.config.Enabled {
		return &SignatureProof{
			Signature:     "unsigned",
			KeyID:         "none",
			SigningMethod: "none",
			Algorithm:     "none",
			Timestamp:     time.Now().UTC(),
			DataHash:      dataHash,
		}, nil
	}

	// Load private key
	privateKey, err := ps.loadPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	// Sign based on key type
	var signature []byte
	switch ps.config.KeyType {
	case SigningKeyRSA:
		signature, err = rsa.SignPKCS1v15(rand.Reader, privateKey.(*rsa.PrivateKey), crypto.SHA256, hash[:])
		if err != nil {
			return nil, fmt.Errorf("RSA signing failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported key type: %s", ps.config.KeyType)
	}

	return &SignatureProof{
		Signature:     base64.StdEncoding.EncodeToString(signature),
		KeyID:         ps.config.KeyID,
		SigningMethod: string(ps.config.KeyType),
		Algorithm:     "SHA256withRSA",
		Timestamp:     time.Now().UTC(),
		DataHash:      dataHash,
	}, nil
}

// VerifySignature verifies a signature against the proof data
func (ps *ProofSigner) VerifySignature(proofData string, signature *SignatureProof) (bool, error) {
	// Compute hash of the proof data
	hash := sha256.Sum256([]byte(proofData))
	dataHash := hex.EncodeToString(hash[:])

	// Check hash matches
	if dataHash != signature.DataHash {
		return false, fmt.Errorf("data hash mismatch: expected %s, got %s", dataHash, signature.DataHash)
	}

	// If signature is "none", verification is not applicable
	if signature.Signature == "unsigned" || signature.SigningMethod == "none" {
		return true, nil
	}

	// Load public key
	publicKey, err := ps.loadPublicKey()
	if err != nil {
		return false, fmt.Errorf("failed to load public key: %w", err)
	}

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(signature.Signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Verify based on algorithm
	switch signature.Algorithm {
	case "SHA256withRSA":
		err = rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA256, hash[:], sigBytes)
		if err != nil {
			return false, fmt.Errorf("RSA verification failed: %w", err)
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported algorithm: %s", signature.Algorithm)
	}
}

// loadPrivateKey loads the private key from the configured path
func (ps *ProofSigner) loadPrivateKey() (crypto.PrivateKey, error) {
	if ps.config.PrivateKeyPath == "" {
		return nil, fmt.Errorf("private key path not configured")
	}

	keyData, err := os.ReadFile(ps.config.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	switch ps.config.KeyType {
	case SigningKeyRSA:
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", ps.config.KeyType)
	}
}

// loadPublicKey loads the public key from the configured path
func (ps *ProofSigner) loadPublicKey() (crypto.PublicKey, error) {
	if ps.config.PublicKeyPath == "" {
		return nil, fmt.Errorf("public key path not configured")
	}

	keyData, err := os.ReadFile(ps.config.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return pub, nil
}

// GenerateSigningKey generates a new RSA key pair for signing proofs
func GenerateSigningKey(bits int, privateKeyPath, publicKeyPath string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Save private key
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	// Save public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0644); err != nil {
		return fmt.Errorf("failed to save public key: %w", err)
	}

	return nil
}

// FormatSignatureProof formats a signature proof for display in markdown
func FormatSignatureProof(proof *SignatureProof) string {
	if proof == nil {
		return "No signature proof available"
	}

	var sb strings.Builder
	sb.WriteString("## Cryptographic Signature\n\n")
	sb.WriteString("| Field | Value |\n")
	sb.WriteString("|-------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Signing Method | `%s` |\n", proof.SigningMethod))
	sb.WriteString(fmt.Sprintf("| Algorithm | `%s` |\n", proof.Algorithm))
	sb.WriteString(fmt.Sprintf("| Key ID | `%s` |\n", proof.KeyID))
	sb.WriteString(fmt.Sprintf("| Timestamp | `%s` |\n", proof.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("| Data Hash (SHA256) | `%s` |\n", proof.DataHash))

	if proof.Signature != "unsigned" && proof.Signature != "" {
		// Truncate signature for readability
		sigDisplay := proof.Signature
		if len(sigDisplay) > 60 {
			sigDisplay = sigDisplay[:30] + "..." + sigDisplay[len(sigDisplay)-27:]
		}
		sb.WriteString(fmt.Sprintf("| Signature | `%s` |\n", sigDisplay))
	} else {
		sb.WriteString("| Signature | `unsigned` |\n")
	}

	sb.WriteString("\n> **Note**: Signature verification requires the public key corresponding to the signing key.\n")

	return sb.String()
}

// SignDirectory signs all proof files in a directory
func (ps *ProofSigner) SignDirectory(dirPath string) (map[string]*SignatureProof, error) {
	signatures := make(map[string]*SignatureProof)

	// Walk the directory and sign all proof files
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only sign proof files (PROOF_OF_WORK.md, .zen-files-changed, etc.)
		if !isProofFile(path) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Sign the content
		signature, err := ps.SignProof(string(content))
		if err != nil {
			// Log warning but continue
			fmt.Printf("Warning: failed to sign file %s: %v\n", path, err)
			return nil
		}

		// Store signature (relative path as key)
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			relPath = path
		}
		signatures[relPath] = signature

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return signatures, nil
}

// isProofFile checks if a file is a proof file that should be signed
func isProofFile(path string) bool {
	base := filepath.Base(path)
	switch base {
	case "PROOF_OF_WORK.md", "PROOF_OF_WORK.json", ".zen-files-changed", ".zen-execution-logs":
		return true
	}
	return strings.HasSuffix(path, ".proof") || strings.HasPrefix(base, "PROOF_")
}
