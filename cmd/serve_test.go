package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func Test_isValidTokenFormat(t *testing.T) {
	validToken := strings.Repeat("a", DefaultSize)

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "valid 64-char nanoid token", token: validToken, want: true},
		{name: "too short", token: "short", want: false},
		{name: "invalid character", token: validToken[:63] + "!", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidTokenFormat(tt.token); got != tt.want {
				t.Errorf("isValidTokenFormat(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func Test_isAllowedStopSignal(t *testing.T) {
	tests := []struct {
		name       string
		signal     string
		wantOK     bool
		wantSignal string
	}{
		{name: "empty defaults to SIGTERM", signal: "", wantOK: true, wantSignal: DefaultStopSignal},
		{name: "QUIT", signal: "QUIT", wantOK: true, wantSignal: "QUIT"},
		{name: "SIGTERM", signal: "SIGTERM", wantOK: true, wantSignal: "SIGTERM"},
		{name: "SIGKILL", signal: "SIGKILL", wantOK: true, wantSignal: "SIGKILL"},
		{name: "invalid signal", signal: "SIGSTOP", wantOK: false, wantSignal: "SIGSTOP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal := tt.signal
			gotOK := isAllowedStopSignal(&signal)
			if gotOK != tt.wantOK {
				t.Errorf("isAllowedStopSignal(%q) ok = %v, want %v", tt.signal, gotOK, tt.wantOK)
			}
			if signal != tt.wantSignal {
				t.Errorf("isAllowedStopSignal signal = %q, want %q", signal, tt.wantSignal)
			}
		})
	}
}

func Test_filterEnvVars(t *testing.T) {
	env := []string{"FOO=bar", "FLOCKMAN_IMAGE_TAG=old", "BAZ=qux", "FLOCKMAN_IMAGE_REPO=repo"}
	got := filterEnvVars(env, "FLOCKMAN_")

	want := []string{"FOO=bar", "BAZ=qux"}
	if len(got) != len(want) {
		t.Fatalf("filterEnvVars() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filterEnvVars()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func Test_repoAndTagFromImage(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		wantRepo string
		wantTag  string
	}{
		{name: "tagged image", image: "nginx:latest", wantRepo: "nginx:", wantTag: "latest"},
		{name: "untagged image", image: "nginx", wantRepo: "nginx:", wantTag: ""},
		{name: "digest stripped", image: "nginx:latest@sha256:abc123", wantRepo: "nginx:", wantTag: "latest"},
		{name: "registry with port", image: "registry:5000/app:v1", wantRepo: "registry:5000/app:", wantTag: "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRepo, gotTag := repoAndTagFromImage(tt.image)
			if gotRepo != tt.wantRepo || gotTag != tt.wantTag {
				t.Errorf("repoAndTagFromImage(%q) = (%q, %q), want (%q, %q)",
					tt.image, gotRepo, gotTag, tt.wantRepo, tt.wantTag)
			}
		})
	}
}

func Test_health(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	health(c)

	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("health status body = %q, want %q", body["status"], "ok")
	}
}
