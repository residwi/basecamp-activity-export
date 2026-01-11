package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Basecamp struct {
	AccountID    string `envconfig:"BASECAMP_ACCOUNT_ID" required:"true"`
	ClientID     string `envconfig:"BASECAMP_CLIENT_ID" required:"true"`
	ClientSecret string `envconfig:"BASECAMP_CLIENT_SECRET" required:"true"`
	RedirectURI  string `envconfig:"BASECAMP_REDIRECT_URI" default:"http://localhost:27072/callback"`
	RateLimit    int    `envconfig:"BASECAMP_RATE_LIMIT" default:"300"` // requests per minute
}

// Config holds the application configuration
type Config struct {
	Basecamp Basecamp
}

func MustLoad() Config {
	godotenv.Load()

	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	return cfg
}

// AppDir returns the application directory path, creating it if necessary
func AppDir() string {
	home, _ := os.UserHomeDir()
	appDir := filepath.Join(home, ".bcreview")

	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		os.MkdirAll(appDir, 0o700)
	}

	return appDir
}

// TokenFile returns the path to the stored token file
func TokenFile() string {
	return filepath.Join(AppDir(), "token.json")
}

// CacheDir returns the path to the cache directory
func CacheDir() string {
	return filepath.Join(AppDir(), "cache")
}
