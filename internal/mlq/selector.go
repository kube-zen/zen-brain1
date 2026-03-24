package mlq

import (
	"fmt"
	"log"
)

// Backend represents an MLQ backend (provider + model combination).

// Backend represents an MLQ backend (provider + model combination).
type Backend struct {
	// ProviderName is LLM provider name (e.g., ollama)
	ProviderName string

	// Model is model name (e.g., qwen3.5:0.8b)
	Model string

	// BaseURL is provider endpoint (e.g., http://host.k3d.internal:11434)
	BaseURL string

	// TimeoutSeconds is request timeout (default 2700s=45m for normal 0.8b lane)
	TimeoutSeconds int

	// EnableThinking indicates if chain-of-thought is enabled
	EnableThinking bool
}

// SelectionCriteria defines how MLQ chooses backend for a task.
type SelectionCriteria struct {
	// PreferredProvider is provider to use if available (empty = MLQ chooses)
	PreferredProvider string

	// MaxContextTokens limits backends to those supporting required context
	MaxContextTokens int

	// RequireTools filters to backends that support function calling
	RequireTools bool

	// TimeoutProfile indicates task type (normal, short, long)
	TimeoutProfile string
}

// MLQ manages backend selection and provider lifecycle.
type MLQ struct {
	// backends are available backends indexed by name
	backends map[string]Backend

	// defaultBackend is fallback if no criteria match
	defaultBackend string
}

// NewMLQ creates a new MLQ instance with registered backends.
func NewMLQ() *MLQ {
	return &MLQ{
		backends:      make(map[string]Backend),
		defaultBackend: "ollama", // Ollama/qwen3.5:0.8b is current working backend
	}
}

// RegisterBackend adds a backend to MLQ.
// ZB-027H: Backends are registered, then MLQ selects based on criteria.
func (m *MLQ) RegisterBackend(name string, backend Backend) {
	m.backends[name] = backend
	log.Printf("[MLQ] Registered backend: name=%s provider=%s model=%s url=%s",
		name, backend.ProviderName, backend.Model, backend.BaseURL)
}

// Select chooses best backend for given criteria.
func (m *MLQ) Select(criteria SelectionCriteria) (*Backend, error) {
	// If PreferredProvider is specified, use it if exists
	if criteria.PreferredProvider != "" {
		if backend, ok := m.backends[criteria.PreferredProvider]; ok {
			log.Printf("[MLQ] Selected preferred backend: %s (provider=%s model=%s)",
				criteria.PreferredProvider, backend.ProviderName, backend.Model)
			return &backend, nil
		}
		log.Printf("[MLQ] Preferred backend '%s' not found, using default", criteria.PreferredProvider)
	}

	// Use default backend
	if backend, ok := m.backends[m.defaultBackend]; ok {
		log.Printf("[MLQ] Selected default backend: %s (provider=%s model=%s)",
			m.defaultBackend, backend.ProviderName, backend.Model)
		return &backend, nil
	}

	return nil, fmt.Errorf("no backend available (default: %s)", m.defaultBackend)
}

// CreateProvider returns provider configuration for factory_runner to create the actual provider.
func (b *Backend) CreateProvider() (providerName, model, baseURL string, timeoutSeconds int) {
	return b.ProviderName, b.Model, b.BaseURL, b.TimeoutSeconds
}

// DefaultConfiguration returns sensible defaults for MLQ setup.
func DefaultConfiguration() *Configuration {
	return &Configuration{
		PrimaryBackend: "ollama",
		FallbackOrder: []string{"ollama"}, // Current working backend only
		Backends: map[string]Backend{
			"ollama": {
				ProviderName:   "ollama",
				Model:         "qwen3.5:0.8b",
				BaseURL:       "http://host.k3d.internal:11434",
				TimeoutSeconds: 2700, // ZB-024: 45 minutes for qwen3.5:0.8b normal lane
				EnableThinking: false,
			},
		},
	}
}

// Configuration holds MLQ backend configuration.
type Configuration struct {
	// PrimaryBackend is default backend
	PrimaryBackend string

	// FallbackOrder is ordered list of backends to try
	FallbackOrder []string

	// Backends is full backend registry
	Backends map[string]Backend
}
