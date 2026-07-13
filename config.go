package main

import (
	"os"
	"strings"

	"github.com/0TrustCloud/search/otrust"
)

type Config struct {
	ListenAddr   string
	DBPath       string
	WALPath      string
	AuthURL      string
	PublicOrigin string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	IndexToken   string // optional shared secret for machine indexing (X-Search-Token)
	SeedOnStart  bool
}

func LoadConfig() Config {
	public := strings.TrimRight(envOr("SEARCH_PUBLIC_URL", "https://search.0trust.cloud"), "/")
	auth := strings.TrimRight(envOr("SEARCH_AUTH_URL", "https://search.0trust.cloud"), "/")
	// Prefer dedicated identity host when set (product IdP on platform).
	if idp := strings.TrimRight(os.Getenv("SEARCH_IDP_URL"), "/"); idp != "" {
		auth = idp
	}
	redirect := os.Getenv("OTRUST_REDIRECT_URI")
	if redirect == "" {
		redirect = public + "/auth/callback"
	}
	return Config{
		ListenAddr:   envOr("SEARCH_LISTEN", ":3087"),
		DBPath:       envOr("SEARCH_DB", "data/search.db"),
		WALPath:      envOr("SEARCH_WAL", "data/search.wal"),
		AuthURL:      auth,
		PublicOrigin: public,
		ClientID:     os.Getenv("OTRUST_CLIENT_ID"),
		ClientSecret: os.Getenv("OTRUST_CLIENT_SECRET"),
		RedirectURI:  redirect,
		IndexToken:   os.Getenv("SEARCH_INDEX_TOKEN"),
		SeedOnStart:  envTruthy("SEARCH_SEED", true),
	}
}

func (c Config) OTrust() otrust.Config {
	return otrust.Config{
		AuthURL:      c.AuthURL,
		PublicOrigin: c.PublicOrigin,
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURI:  c.RedirectURI,
		ServiceName:  "search",
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envTruthy(key string, defaultVal bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultVal
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultVal
	}
}
