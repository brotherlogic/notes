package main

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port               int
	DataDir            string
	FrontendDir        string
	GitHubClientID     string
	GitHubClientSecret string
	GDriveClientID     string
	GDriveClientSecret string
	PStoreAddress      string
}

func LoadConfig() (*Config, error) {
	portStr := os.Getenv("PORT")
	port := 8080
	if portStr != "" {
		if val, err := strconv.Atoi(portStr); err == nil {
			port = val
		}
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data/binaries"
	}

	frontendDir := os.Getenv("FRONTEND_DIR")
	if frontendDir == "" {
		frontendDir = "./frontend/dist"
	}

	ghClientID := os.Getenv("GITHUB_CLIENT_ID")
	ghClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	gdClientID := os.Getenv("GDRIVE_CLIENT_ID")
	gdClientSecret := os.Getenv("GDRIVE_CLIENT_SECRET")
	pstoreAddr := os.Getenv("PSTORE_ADDRESS")

	// Validate required variables for hosting
	if ghClientID == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_ID is required")
	}
	if ghClientSecret == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_SECRET is required")
	}
	if gdClientID == "" {
		return nil, fmt.Errorf("GDRIVE_CLIENT_ID is required")
	}
	if gdClientSecret == "" {
		return nil, fmt.Errorf("GDRIVE_CLIENT_SECRET is required")
	}
	if pstoreAddr == "" {
		return nil, fmt.Errorf("PSTORE_ADDRESS is required")
	}

	return &Config{
		Port:               port,
		DataDir:            dataDir,
		FrontendDir:        frontendDir,
		GitHubClientID:     ghClientID,
		GitHubClientSecret: ghClientSecret,
		GDriveClientID:     gdClientID,
		GDriveClientSecret: gdClientSecret,
		PStoreAddress:      pstoreAddr,
	}, nil
}
