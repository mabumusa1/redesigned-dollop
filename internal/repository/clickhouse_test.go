package repository

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConnectionConfig(t *testing.T) {
	cfg := DefaultConnectionConfig()

	if len(cfg.Hosts) != 1 || cfg.Hosts[0] != "localhost:9000" {
		t.Errorf("unexpected hosts: %v", cfg.Hosts)
	}
	if cfg.Database != "fanfinity" {
		t.Errorf("expected database fanfinity, got %s", cfg.Database)
	}
	if cfg.Username != "default" {
		t.Errorf("expected username default, got %s", cfg.Username)
	}
	if cfg.Password != "" {
		t.Error("expected empty password")
	}
	if cfg.MaxOpenConns != 10 {
		t.Errorf("expected max open conns 10, got %d", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 5 {
		t.Errorf("expected max idle conns 5, got %d", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != time.Hour {
		t.Errorf("expected conn max lifetime 1h, got %v", cfg.ConnMaxLifetime)
	}
	if cfg.DialTimeout != 10*time.Second {
		t.Errorf("expected dial timeout 10s, got %v", cfg.DialTimeout)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", cfg.ReadTimeout)
	}
	if cfg.Debug != false {
		t.Error("expected debug false")
	}
}

func TestConnectionConfig_CustomValues(t *testing.T) {
	cfg := ConnectionConfig{
		Hosts:           []string{"ch1:9000", "ch2:9000"},
		Database:        "custom_db",
		Username:        "admin",
		Password:        "secret",
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: 2 * time.Hour,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     60 * time.Second,
		Debug:           true,
	}

	if len(cfg.Hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(cfg.Hosts))
	}
	if cfg.Database != "custom_db" {
		t.Errorf("expected database custom_db, got %s", cfg.Database)
	}
	if cfg.Username != "admin" {
		t.Errorf("expected username admin, got %s", cfg.Username)
	}
	if cfg.Password != "secret" {
		t.Errorf("expected password secret, got %s", cfg.Password)
	}
	if cfg.MaxOpenConns != 20 {
		t.Errorf("expected max open conns 20, got %d", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("expected max idle conns 10, got %d", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 2*time.Hour {
		t.Errorf("expected conn max lifetime 2h, got %v", cfg.ConnMaxLifetime)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("expected dial timeout 5s, got %v", cfg.DialTimeout)
	}
	if cfg.ReadTimeout != 60*time.Second {
		t.Errorf("expected read timeout 60s, got %v", cfg.ReadTimeout)
	}
	if cfg.Debug != true {
		t.Error("expected debug true")
	}
}

func TestNewClickHouseRepository_NilLogger(t *testing.T) {
	// Pass nil connection and nil logger
	repo := NewClickHouseRepository(nil, nil)

	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
	if repo.logger == nil {
		t.Error("expected default logger to be set")
	}
}

func TestClickHouseRepository_Close_NilConnection(t *testing.T) {
	repo := &ClickHouseRepository{
		conn: nil,
	}

	err := repo.Close()
	if err != nil {
		t.Errorf("expected nil error for nil connection, got: %v", err)
	}
}

func TestClickHouseRepository_InsertBatch_EmptyBatch(t *testing.T) {
	repo := NewClickHouseRepository(nil, nil)

	// Empty batch should return nil without error
	err := repo.InsertBatch(context.Background(), nil)
	if err != nil {
		t.Errorf("expected nil error for empty batch, got: %v", err)
	}
}

func TestClickHouseRepository_GetMatchMetrics_EmptyMatchID(t *testing.T) {
	repo := NewClickHouseRepository(nil, nil)

	metrics, err := repo.GetMatchMetrics(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty matchID")
	}
	if metrics != nil {
		t.Error("expected nil metrics for empty matchID")
	}
	if err.Error() != "matchID cannot be empty" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestClickHouseRepository_GetEventsPerMinute_EmptyMatchID(t *testing.T) {
	repo := NewClickHouseRepository(nil, nil)

	events, err := repo.GetEventsPerMinute(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty matchID")
	}
	if events != nil {
		t.Error("expected nil events for empty matchID")
	}
	if err.Error() != "matchID cannot be empty" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func BenchmarkDefaultConnectionConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultConnectionConfig()
	}
}
