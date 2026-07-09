package app

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv                    string
	HTTPAddr                  string
	FrontendURL               string
	MoodleLoginURL            string
	MoodleOTASignLaunchURL    string
	MoodleLaunchSigningSecret string
	DatabaseMigrationsPath    string
	SessionCookieName         string
	SessionCookieSecure       bool
	EnforceHTTPS              bool
	DatabaseURL               string
	DocuSealURL               string
	DocuSealPublicURL         string
	DocuSealAPIKey            string
	DocuSealWebhookSecret     string
	DocuSealWebhookMaxAge     int
	NotificationWebhookURL    string
	NotificationWebhookSecret string
}

func LoadConfig() Config {
	return Config{
		AppEnv:                    env("APP_ENV", "development"),
		HTTPAddr:                  env("HTTP_ADDR", ":8080"),
		FrontendURL:               strings.TrimRight(env("FRONTEND_URL", "http://localhost:5173"), "/"),
		MoodleLoginURL:            env("MOODLE_LOGIN_URL", "http://localhost/login/index.php"),
		MoodleOTASignLaunchURL:    env("MOODLE_OTA_SIGN_LAUNCH_URL", ""),
		MoodleLaunchSigningSecret: env("MOODLE_LAUNCH_SIGNING_SECRET", "dev-only-change-me"),
		DatabaseMigrationsPath:    env("DATABASE_MIGRATIONS_PATH", "db/migrations"),
		SessionCookieName:         env("SESSION_COOKIE_NAME", "otasign_session"),
		SessionCookieSecure:       envBool("SESSION_COOKIE_SECURE", false),
		EnforceHTTPS:              envBool("ENFORCE_HTTPS", false),
		DatabaseURL:               env("DATABASE_URL", ""),
		DocuSealURL:               strings.TrimRight(env("DOCUSEAL_URL", ""), "/"),
		DocuSealPublicURL:         strings.TrimRight(env("DOCUSEAL_PUBLIC_URL", env("DOCUSEAL_URL", "")), "/"),
		DocuSealAPIKey:            env("DOCUSEAL_API_KEY", ""),
		DocuSealWebhookSecret:     env("DOCUSEAL_WEBHOOK_SECRET", ""),
		DocuSealWebhookMaxAge:     envInt("DOCUSEAL_WEBHOOK_MAX_AGE_SECONDS", 300),
		NotificationWebhookURL:    env("NOTIFICATION_WEBHOOK_URL", ""),
		NotificationWebhookSecret: env("NOTIFICATION_WEBHOOK_SECRET", ""),
	}
}

func (c Config) Validate() error {
	if strings.EqualFold(c.AppEnv, "production") {
		if c.DatabaseURL == "" {
			return configError("DATABASE_URL is required in production")
		}
		if c.FrontendURL == "" || !strings.HasPrefix(c.FrontendURL, "https://") || strings.HasPrefix(c.FrontendURL, "http://localhost") {
			return configError("FRONTEND_URL must be a production HTTPS URL")
		}
		if c.MoodleLoginURL == "" || !strings.HasPrefix(c.MoodleLoginURL, "https://") || strings.HasPrefix(c.MoodleLoginURL, "http://localhost") {
			return configError("MOODLE_LOGIN_URL must be a production HTTPS URL")
		}
		if c.MoodleLaunchSigningSecret == "" || c.MoodleLaunchSigningSecret == "dev-only-change-me" {
			return configError("MOODLE_LAUNCH_SIGNING_SECRET must be set to a production secret")
		}
		if !c.SessionCookieSecure {
			return configError("SESSION_COOKIE_SECURE must be true in production")
		}
		if !c.EnforceHTTPS {
			return configError("ENFORCE_HTTPS must be true in production")
		}
		if c.DocuSealURL == "" || c.DocuSealAPIKey == "" {
			return configError("DOCUSEAL_URL and DOCUSEAL_API_KEY are required in production")
		}
		if c.DocuSealWebhookSecret == "" {
			return configError("DOCUSEAL_WEBHOOK_SECRET is required in production")
		}
	}

	return nil
}

type configError string

func (e configError) Error() string {
	return string(e)
}

func env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
