// Package config reads process settings from the environment.
// Secrets are never given a default and are never written by this package.
package config

import (
	"os"
	"strconv"
	"time"
)

// ClaimNames are the token and directory attribute names that carry org data.
// Active Directory remains the source. Keycloak is how the portal reads them.
type ClaimNames struct {
	Email          string
	Locale         string
	EmployeeNumber string
	Position       string
	Department     string
	Manager        string
	HireDate       string
}

// Config is the process configuration. Empty strings mean "not supplied".
type Config struct {
	HTTPAddr string

	DatabaseURL string

	KeycloakIssuer       string
	KeycloakClientID     string
	KeycloakClientSecret string
	KeycloakHRGroup      string

	KeycloakAdminClientID     string
	KeycloakAdminClientSecret string

	SMTPHost     string
	SMTPPort     int
	SMTPTLS      string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	Claims ClaimNames

	SyncInterval   time.Duration
	NotifyInterval time.Duration
	CORSOrigin     string
}

func Load() Config {
	cfg := Config{
		HTTPAddr:                  getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		KeycloakIssuer:            os.Getenv("KEYCLOAK_ISSUER"),
		KeycloakClientID:          os.Getenv("KEYCLOAK_CLIENT_ID"),
		KeycloakClientSecret:      os.Getenv("KEYCLOAK_CLIENT_SECRET"),
		KeycloakHRGroup:           getenv("KEYCLOAK_HR_GROUP", "hr"),
		KeycloakAdminClientID:     os.Getenv("KEYCLOAK_ADMIN_CLIENT_ID"),
		KeycloakAdminClientSecret: os.Getenv("KEYCLOAK_ADMIN_CLIENT_SECRET"),
		SMTPHost:                  os.Getenv("SMTP_HOST"),
		SMTPPort:                  getenvInt("SMTP_PORT", 587),
		SMTPTLS:                   getenv("SMTP_TLS", "starttls"),
		SMTPUser:                  os.Getenv("SMTP_USER"),
		SMTPPassword:              os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:                  os.Getenv("SMTP_FROM"),
		Claims: ClaimNames{
			Email:          getenv("CLAIM_EMAIL", "email"),
			Locale:         getenv("CLAIM_LOCALE", "locale"),
			EmployeeNumber: getenv("CLAIM_EMPLOYEE_NUMBER", "employee_number"),
			Position:       getenv("CLAIM_POSITION", "position"),
			Department:     getenv("CLAIM_DEPARTMENT", "department"),
			Manager:        getenv("CLAIM_MANAGER", "manager"),
			HireDate:       getenv("CLAIM_HIRE_DATE", "hire_date"),
		},
		SyncInterval:   getenvDuration("SYNC_INTERVAL", 15*time.Minute),
		NotifyInterval: getenvDuration("NOTIFY_INTERVAL", time.Minute),
		CORSOrigin:     os.Getenv("CORS_ORIGIN"),
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
