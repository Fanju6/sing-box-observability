package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAMLDefaultsAndDurations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yaml")
	if err := os.WriteFile(path, []byte("collector:\n  scrape_interval: 2s\n  persist_interval: 15s\nstorage:\n  retention: 168h\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsUnauthenticatedNonLoopbackListen(t *testing.T) {
	cfg := Default()
	cfg.Server.Listen = "0.0.0.0:9095"
	if err := Validate(&cfg); err == nil {
		t.Fatal("expected non-loopback listener without console authentication to be rejected")
	}
	cfg.Console.AuthToken = "configured"
	if err := Validate(&cfg); err != nil {
		t.Fatalf("authenticated non-loopback listener was rejected: %v", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yaml")
	if err := os.WriteFile(path, []byte("server:\n  listen: 127.0.0.1:9095\n  typo: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown YAML field to be rejected")
	}
}

func TestLoadRejectsMultipleDocuments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yaml")
	if err := os.WriteFile(path, []byte("server:\n  listen: 127.0.0.1:9095\n---\nserver:\n  listen: 127.0.0.1:9096\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected multiple YAML documents to be rejected")
	}
}
