package main

import (
	"os"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	// Setup environment variables
	os.Setenv("PORT", "9090")
	os.Setenv("DATA_DIR", "/tmp/notes-bin")
	os.Setenv("FRONTEND_DIR", "/tmp/notes-web")
	os.Setenv("GITHUB_CLIENT_ID", "gh-123")
	os.Setenv("GITHUB_CLIENT_SECRET", "gh-sec")
	os.Setenv("GDRIVE_CLIENT_ID", "gd-123")
	os.Setenv("GDRIVE_CLIENT_SECRET", "gd-sec")
	os.Setenv("PSTORE_ADDRESS", "localhost:50051")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATA_DIR")
		os.Unsetenv("FRONTEND_DIR")
		os.Unsetenv("GITHUB_CLIENT_ID")
		os.Unsetenv("GITHUB_CLIENT_SECRET")
		os.Unsetenv("GDRIVE_CLIENT_ID")
		os.Unsetenv("GDRIVE_CLIENT_SECRET")
		os.Unsetenv("PSTORE_ADDRESS")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/notes-bin" {
		t.Errorf("Expected DataDir /tmp/notes-bin, got %s", cfg.DataDir)
	}
	if cfg.FrontendDir != "/tmp/notes-web" {
		t.Errorf("Expected FrontendDir /tmp/notes-web, got %s", cfg.FrontendDir)
	}
	if cfg.GitHubClientID != "gh-123" {
		t.Errorf("Expected GitHubClientID gh-123, got %s", cfg.GitHubClientID)
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	os.Unsetenv("GITHUB_CLIENT_ID")
	_, err := LoadConfig()
	if err == nil {
		t.Error("Expected error due to missing GITHUB_CLIENT_ID, got nil")
	}
}
