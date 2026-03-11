package runtime

import (
	"sync"
)

// CircuitBreakerRegistry provides global access to circuit breakers.
// This allows doctor/preflight checks to query circuit breaker states.
type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// globalRegistry is the global circuit breaker registry.
var globalRegistry = &CircuitBreakerRegistry{
	breakers: make(map[string]*CircuitBreaker),
}

// GetCircuitBreakerRegistry returns the global circuit breaker registry.
func GetCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return globalRegistry
}

// Register adds a circuit breaker to the registry.
func (r *CircuitBreakerRegistry) Register(name string, cb *CircuitBreaker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.breakers[name] = cb
}

// Unregister removes a circuit breaker from the registry.
func (r *CircuitBreakerRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.breakers, name)
}

// Get retrieves a circuit breaker by name.
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.breakers[name]
}

// GetAll returns all circuit breakers.
func (r *CircuitBreakerRegistry) GetAll() map[string]*CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*CircuitBreaker, len(r.breakers))
	for k, v := range r.breakers {
		result[k] = v
	}
	return result
}

// GetAllStates returns the state of all circuit breakers.
func (r *CircuitBreakerRegistry) GetAllStates() map[string]CircuitBreakerState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]CircuitBreakerState, len(r.breakers))
	for name, cb := range r.breakers {
		result[name] = CircuitBreakerState{
			Name:     name,
			State:    cb.State(),
			Failures: cb.Failures(),
			Healthy:  cb.State() == CircuitStateClosed || cb.State() == CircuitStateHalfOpen,
		}
	}
	return result
}

// IsHealthy returns true if all circuit breakers are healthy (closed or half-open).
func (r *CircuitBreakerRegistry) IsHealthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cb := range r.breakers {
		if cb.State() == CircuitStateOpen {
			return false
		}
	}
	return true
}

// GetUnhealthy returns names of circuit breakers in open state.
func (r *CircuitBreakerRegistry) GetUnhealthy() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var unhealthy []string
	for name, cb := range r.breakers {
		if cb.State() == CircuitStateOpen {
			unhealthy = append(unhealthy, name)
		}
	}
	return unhealthy
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState struct {
	Name     string        `json:"name"`
	State    CircuitState  `json:"state"`
	Failures int           `json:"failures"`
	Healthy  bool          `json:"healthy"`
}

// RegisterCircuitBreaker registers a circuit breaker with the global registry.
func RegisterCircuitBreaker(name string, cb *CircuitBreaker) {
	globalRegistry.Register(name, cb)
}

// UnregisterCircuitBreaker unregisters a circuit breaker from the global registry.
func UnregisterCircuitBreaker(name string) {
	globalRegistry.Unregister(name)
}

// GetCircuitBreaker retrieves a circuit breaker from the global registry.
func GetCircuitBreaker(name string) *CircuitBreaker {
	return globalRegistry.Get(name)
}

// GetAllCircuitBreakerStates returns all circuit breaker states from the global registry.
func GetAllCircuitBreakerStates() map[string]CircuitBreakerState {
	return globalRegistry.GetAllStates()
}

// AreCircuitBreakersHealthy returns true if all circuit breakers are healthy.
func AreCircuitBreakersHealthy() bool {
	return globalRegistry.IsHealthy()
}

// GetOpenCircuitBreakers returns names of open circuit breakers.
func GetOpenCircuitBreakers() []string {
	return globalRegistry.GetUnhealthy()
}
