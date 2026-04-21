package main

import (
	"flag"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestDefaultLocalModelEnabled(t *testing.T) {
	original := DefaultLocalModel
	t.Cleanup(func() {
		DefaultLocalModel = original
	})

	DefaultLocalModel = "true"
	if !defaultLocalModelEnabled() {
		t.Fatalf("defaultLocalModelEnabled() = false, want true")
	}

	DefaultLocalModel = "false"
	if defaultLocalModelEnabled() {
		t.Fatalf("defaultLocalModelEnabled() = true, want false")
	}
}

func TestModelCatalogCLIOverrideFromFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		localModel  bool
		remoteModel bool
		want        string
	}{
		{
			name:       "no explicit override",
			args:       []string{},
			localModel: true,
			want:       "",
		},
		{
			name:       "local model forced",
			args:       []string{"-local-model"},
			localModel: true,
			want:       config.ModelCatalogCLIOverrideLocal,
		},
		{
			name:        "remote model forced",
			args:        []string{"-remote-model"},
			remoteModel: true,
			want:        config.ModelCatalogCLIOverrideRemote,
		},
		{
			name:       "explicit false does not force local mode",
			args:       []string{"-local-model=false"},
			localModel: false,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			localModel := tt.localModel
			remoteModel := tt.remoteModel
			flags.BoolVar(&localModel, "local-model", tt.localModel, "")
			flags.BoolVar(&remoteModel, "remote-model", tt.remoteModel, "")
			if err := flags.Parse(tt.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			if remoteModel {
				localModel = false
			}

			got := modelCatalogCLIOverrideFromFlags(flags, localModel, remoteModel)
			if got != tt.want {
				t.Fatalf("modelCatalogCLIOverrideFromFlags() = %q, want %q", got, tt.want)
			}
		})
	}
}
