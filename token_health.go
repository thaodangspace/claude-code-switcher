package main

import "time"

// TokenHealthStatus is an offline, local classification of a saved OAuth
// credential. It is not a server-side validity check.
type TokenHealthStatus string

const (
	TokenReady         TokenHealthStatus = "ready"
	TokenRefreshNeeded TokenHealthStatus = "refresh-needed"
	TokenRelogin       TokenHealthStatus = "re-login"
	TokenUnknown       TokenHealthStatus = "unknown"
)

// TokenHealth is the secret-free result of local credential classification.
type TokenHealth struct {
	Status           TokenHealthStatus `json:"status"`
	Detail           string            `json:"detail,omitempty"`
	AccessExpiresAt  *time.Time        `json:"accessExpiresAt,omitempty"`
	RefreshExpiresAt *time.Time        `json:"refreshExpiresAt,omitempty"`
}

// EvaluateTokenHealth classifies an OAuth credential without filesystem,
// process, network, or refresh behavior. Expiry values are Unix milliseconds.
func EvaluateTokenHealth(c *ClaudeAIOAuthCredential, now time.Time) TokenHealth {
	if c == nil {
		return TokenHealth{Status: TokenUnknown, Detail: "credential not saved"}
	}
	if c.ExpiresAt < 0 || c.RefreshTokenExpiresAt < 0 {
		return TokenHealth{Status: TokenUnknown, Detail: "unsupported expiry"}
	}

	health := TokenHealth{}
	if c.ExpiresAt > 0 {
		expires := time.UnixMilli(c.ExpiresAt)
		health.AccessExpiresAt = &expires
	}
	if c.RefreshTokenExpiresAt > 0 {
		expires := time.UnixMilli(c.RefreshTokenExpiresAt)
		health.RefreshExpiresAt = &expires
	}

	if c.AccessToken != "" && c.ExpiresAt > now.UnixMilli() {
		health.Status = TokenReady
		health.Detail = "access token valid"
		if c.RefreshToken == "" {
			health.Detail = "no refresh token"
		}
		return health
	}

	if c.RefreshToken != "" && (c.RefreshTokenExpiresAt == 0 || c.RefreshTokenExpiresAt > now.UnixMilli()) {
		health.Status = TokenRefreshNeeded
		if c.RefreshTokenExpiresAt == 0 {
			health.Detail = "refresh expiry unknown"
		} else {
			health.Detail = "refresh token locally unexpired"
		}
		return health
	}

	health.Status = TokenRelogin
	if c.RefreshToken == "" {
		health.Detail = "no refresh token"
	} else {
		health.Detail = "refresh token expired"
	}
	return health
}

// formatTokenDuration formats a future expiry as a compact, floored relative
// duration suitable for human list output.
func formatTokenDuration(now, expiry time.Time) string {
	duration := expiry.Sub(now)
	if duration <= 0 {
		return "0s"
	}
	seconds := int64(duration / time.Second)
	if seconds < 1 {
		return "0s"
	}
	if days := seconds / (24 * 60 * 60); days > 0 {
		return formatCountUnit(days, "d")
	}
	if hours := seconds / (60 * 60); hours > 0 {
		return formatCountUnit(hours, "h")
	}
	if minutes := seconds / 60; minutes > 0 {
		return formatCountUnit(minutes, "m")
	}
	return formatCountUnit(seconds, "s")
}

func formatCountUnit(count int64, unit string) string {
	return formatInt64(count) + unit
}

func formatInt64(value int64) string {
	// Avoid fmt in the evaluator's hot path while keeping this helper tiny.
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}
