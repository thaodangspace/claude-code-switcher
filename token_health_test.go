package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEvaluateTokenHealth(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	future := now.Add(2 * time.Hour).UnixMilli()
	past := now.Add(-time.Hour).UnixMilli()
	cases := []struct {
		name   string
		cred   *ClaudeAIOAuthCredential
		status TokenHealthStatus
		detail string
	}{
		{"future access", &ClaudeAIOAuthCredential{AccessToken: "a", ExpiresAt: future}, TokenReady, "no refresh token"},
		{"expired access future refresh", &ClaudeAIOAuthCredential{RefreshToken: "r", ExpiresAt: past, RefreshTokenExpiresAt: future}, TokenRefreshNeeded, "refresh token locally unexpired"},
		{"zero access expiry", &ClaudeAIOAuthCredential{RefreshToken: "r"}, TokenRefreshNeeded, "refresh expiry unknown"},
		{"missing access valid refresh", &ClaudeAIOAuthCredential{RefreshToken: "r", RefreshTokenExpiresAt: future}, TokenRefreshNeeded, "refresh token locally unexpired"},
		{"expired refresh", &ClaudeAIOAuthCredential{AccessToken: "a", ExpiresAt: past, RefreshToken: "r", RefreshTokenExpiresAt: past}, TokenRelogin, "refresh token expired"},
		{"missing refresh after access", &ClaudeAIOAuthCredential{AccessToken: "a", ExpiresAt: past}, TokenRelogin, "no refresh token"},
		{"future access missing refresh", &ClaudeAIOAuthCredential{AccessToken: "a", ExpiresAt: future}, TokenReady, "no refresh token"},
		{"exact boundary", &ClaudeAIOAuthCredential{AccessToken: "a", ExpiresAt: now.UnixMilli(), RefreshToken: "r", RefreshTokenExpiresAt: future}, TokenRefreshNeeded, "refresh token locally unexpired"},
		{"nil", nil, TokenUnknown, "credential not saved"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateTokenHealth(tc.cred, now)
			if got.Status != tc.status || got.Detail != tc.detail {
				t.Fatalf("got %#v, want status=%q detail=%q", got, tc.status, tc.detail)
			}
		})
	}
}

func TestClaudeAIOAuthCredentialRejectsUnsupportedExpiry(t *testing.T) {
	var credential ClaudeAIOAuthCredential
	err := json.Unmarshal([]byte(`{"accessToken":"secret","expiresAt":"tomorrow"}`), &credential)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe unsupported expiry error: %v", err)
	}
}

func TestFormatTokenDuration(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	for _, tc := range []struct {
		d time.Duration
		w string
	}{
		{6*time.Hour + 21*time.Minute + 59*time.Second, "6h"},
		{19*24*time.Hour + time.Hour, "19d"},
		{45 * time.Second, "45s"},
	} {
		if got := formatTokenDuration(now, now.Add(tc.d)); got != tc.w {
			t.Errorf("duration %s = %q, want %q", tc.d, got, tc.w)
		}
	}
}
