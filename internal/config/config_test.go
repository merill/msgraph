package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadCloudEndpoints(t *testing.T) {
	tests := []struct {
		name          string
		cloud         string
		unset         bool
		wantCloud     Cloud
		wantAuthority string
		wantGraphURL  string
	}{
		{
			name:          "unset uses global",
			cloud:         "temporary",
			unset:         true,
			wantCloud:     CloudGlobal,
			wantAuthority: "https://login.microsoftonline.com/",
			wantGraphURL:  "https://graph.microsoft.com/v1.0",
		},
		{
			name:          "blank uses global",
			cloud:         "   ",
			wantCloud:     CloudGlobal,
			wantAuthority: "https://login.microsoftonline.com/",
			wantGraphURL:  "https://graph.microsoft.com/v1.0",
		},
		{
			name:          "US government",
			cloud:         "usgov",
			wantCloud:     CloudUSGov,
			wantAuthority: "https://login.microsoftonline.us/",
			wantGraphURL:  "https://graph.microsoft.us/v1.0",
		},
		{
			name:          "US government DoD",
			cloud:         "usgovdod",
			wantCloud:     CloudUSGovDoD,
			wantAuthority: "https://login.microsoftonline.us/",
			wantGraphURL:  "https://dod-graph.microsoft.us/v1.0",
		},
		{
			name:          "China",
			cloud:         "china",
			wantCloud:     CloudChina,
			wantAuthority: "https://login.chinacloudapi.cn/",
			wantGraphURL:  "https://microsoftgraph.chinacloudapi.cn/v1.0",
		},
		{
			name:          "case insensitive",
			cloud:         "USGov",
			wantCloud:     CloudUSGov,
			wantAuthority: "https://login.microsoftonline.us/",
			wantGraphURL:  "https://graph.microsoft.us/v1.0",
		},
		{
			name:          "whitespace tolerant",
			cloud:         " usgov ",
			wantCloud:     CloudUSGov,
			wantAuthority: "https://login.microsoftonline.us/",
			wantGraphURL:  "https://graph.microsoft.us/v1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MSGRAPH_CLOUD", tt.cloud)
			if tt.unset {
				if err := os.Unsetenv("MSGRAPH_CLOUD"); err != nil {
					t.Fatalf("os.Unsetenv() error = %v", err)
				}
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Cloud != tt.wantCloud {
				t.Errorf("Cloud = %q, want %q", cfg.Cloud, tt.wantCloud)
			}
			if cfg.Authority != tt.wantAuthority {
				t.Errorf("Authority = %q, want %q", cfg.Authority, tt.wantAuthority)
			}
			if got := cfg.GraphURL("v1.0"); got != tt.wantGraphURL {
				t.Errorf("GraphURL(\"v1.0\") = %q, want %q", got, tt.wantGraphURL)
			}

			wantGraphBaseURL := strings.TrimSuffix(tt.wantGraphURL, "/v1.0")
			if got := cfg.GraphDefaultScope(); got != wantGraphBaseURL+"/.default" {
				t.Errorf("GraphDefaultScope() = %q, want %q", got, wantGraphBaseURL+"/.default")
			}
			if got := cfg.GraphResource(); got != wantGraphBaseURL {
				t.Errorf("GraphResource() = %q, want %q", got, wantGraphBaseURL)
			}
		})
	}
}

func TestLoadRejectsInvalidCloud(t *testing.T) {
	t.Setenv("MSGRAPH_CLOUD", "gcchigh")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want an invalid cloud error")
	}
	if !strings.Contains(err.Error(), "gcchigh") || !strings.Contains(err.Error(), "usgov") {
		t.Errorf("Load() error = %q, want bad value and valid values", err)
	}
}

func TestAuthorityURLForNationalCloud(t *testing.T) {
	t.Setenv("MSGRAPH_CLOUD", "china")
	t.Setenv("MSGRAPH_TENANT_ID", "contoso.onmicrosoft.cn")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	const want = "https://login.chinacloudapi.cn/contoso.onmicrosoft.cn"
	if got := cfg.AuthorityURL(); got != want {
		t.Errorf("AuthorityURL() = %q, want %q", got, want)
	}
}
