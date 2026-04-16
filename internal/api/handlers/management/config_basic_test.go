package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveVersionFileURL(t *testing.T) {
	t.Helper()

	tests := []struct {
		name       string
		releaseURL string
		want       string
	}{
		{
			name:       "github api release url",
			releaseURL: "https://api.github.com/repos/lauvww/CLIProxyAPI/releases/latest",
			want:       "https://raw.githubusercontent.com/lauvww/CLIProxyAPI/main/VERSION",
		},
		{
			name:       "github repo url",
			releaseURL: "https://github.com/lauvww/CLIProxyAPI",
			want:       "https://raw.githubusercontent.com/lauvww/CLIProxyAPI/main/VERSION",
		},
		{
			name:       "invalid url",
			releaseURL: "::://invalid",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveVersionFileURL(tt.releaseURL)
			if got != tt.want {
				t.Fatalf("resolveVersionFileURL(%q) = %q, want %q", tt.releaseURL, got, tt.want)
			}
		})
	}
}

func TestFetchLatestVersionFromRelease(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	defer server.Close()

	version, err := fetchLatestVersionFromRelease(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("fetchLatestVersionFromRelease returned error: %v", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("version = %q, want %q", version, "v1.2.3")
	}
}

func TestFetchLatestVersionFromFile(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.0.0\n"))
	}))
	defer server.Close()

	version, err := fetchLatestVersionFromFile(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("fetchLatestVersionFromFile returned error: %v", err)
	}
	if version != "1.0.0" {
		t.Fatalf("version = %q, want %q", version, "1.0.0")
	}
}

func TestFetchLatestVersionFallsBackToVersionFile(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/lauvww/CLIProxyAPI/releases/latest":
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
		case "/lauvww/CLIProxyAPI/main/VERSION":
			_, _ = w.Write([]byte("1.0.0"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	releaseURL := server.URL + "/repos/lauvww/CLIProxyAPI/releases/latest"
	version, err := fetchLatestVersionFromRelease(context.Background(), server.Client(), releaseURL)
	if err == nil {
		t.Fatal("fetchLatestVersionFromRelease error = nil, want error")
	}

	versionFileURL := resolveVersionFileURL("https://api.github.com/repos/lauvww/CLIProxyAPI/releases/latest")
	versionFileURL = strings.Replace(versionFileURL, "https://raw.githubusercontent.com", server.URL, 1)

	version, err = fetchLatestVersionFromFile(context.Background(), server.Client(), versionFileURL)
	if err != nil {
		t.Fatalf("fetchLatestVersionFromFile returned error: %v", err)
	}
	if version != "1.0.0" {
		t.Fatalf("version = %q, want %q", version, "1.0.0")
	}
}
