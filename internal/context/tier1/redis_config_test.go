package tier1

import (
	"testing"
)

// TestDefaultRedisConfig tests the default configuration is properly initialized.
func TestDefaultRedisConfig(t *testing.T) {
	config := DefaultRedisConfig()

	if config.URL != "" {
		t.Errorf("URL should be empty, got: %s", config.URL)
	}
	if config.Addr != "localhost:6379" {
		t.Errorf("Addr should be localhost:6379, got: %s", config.Addr)
	}
	if config.DB != 0 {
		t.Errorf("DB should be 0, got: %d", config.DB)
	}
	if config.PoolSize != 10 {
		t.Errorf("PoolSize should be 10, got: %d", config.PoolSize)
	}
	if config.MinIdleConns != 3 {
		t.Errorf("MinIdleConns should be 3, got: %d", config.MinIdleConns)
	}
	if config.MaxRetries != 3 {
		t.Errorf("MaxRetries should be 3, got: %d", config.MaxRetries)
	}
	if config.DialTimeout.Seconds() != 5 {
		t.Errorf("DialTimeout should be 5s, got: %v", config.DialTimeout)
	}
	if config.ReadTimeout.Seconds() != 3 {
		t.Errorf("ReadTimeout should be 3s, got: %v", config.ReadTimeout)
	}
	if config.WriteTimeout.Seconds() != 3 {
		t.Errorf("WriteTimeout should be 3s, got: %v", config.WriteTimeout)
	}
	if config.PoolTimeout.Seconds() != 4 {
		t.Errorf("PoolTimeout should be 4s, got: %v", config.PoolTimeout)
	}
	if config.IdleTimeout.Minutes() != 5 {
		t.Errorf("IdleTimeout should be 5m, got: %v", config.IdleTimeout)
	}
	if config.IdleCheckFrequency.Minutes() != 1 {
		t.Errorf("IdleCheckFrequency should be 1m, got: %v", config.IdleCheckFrequency)
	}
	if config.TLSEnabled != false {
		t.Errorf("TLSEnabled should be false, got: %v", config.TLSEnabled)
	}
}
