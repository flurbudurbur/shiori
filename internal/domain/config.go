package domain

// ServerConfig holds server-related settings
type ServerConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"` // Changed tag from baseUrl to base_url
}

// PostgresConfig holds PostgreSQL-specific settings
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"` // Kept as int, Viper should handle string conversion
	Database string `mapstructure:"database"`
	User     string `mapstructure:"username"`
	Pass     string `mapstructure:"password"`
	SslMode  string `mapstructure:"ssl_mode"` // Changed tag from postgresSslMode to ssl_mode
}

// DatabaseConfig holds general database settings and nested specific configs
type DatabaseConfig struct {
	Type     string         `mapstructure:"type"`
	Postgres PostgresConfig `mapstructure:"postgres"` // Nested struct for [database.postgres]
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Path           string `mapstructure:"path"`
	Level          string `mapstructure:"level"`            // Changed tag from logLevel to level
	MaxFileSize    int    `mapstructure:"max_file_size"`    // Changed tag from max_file_size (was already correct)
	MaxBackupCount int    `mapstructure:"max_backup_count"` // Changed tag from max_backup_count (was already correct)
}

// ValkeyConfig holds Valkey-specific settings
type ValkeyConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	RequestsPerMinute int    `mapstructure:"requests_per_minute"`
	WindowSeconds     int    `mapstructure:"window_seconds"`
	ExemptRoles       string `mapstructure:"exempt_roles"`
	ExemptInternalIPs string `mapstructure:"exempt_internal_ips"`
}

// UUIDCleanupConfig holds settings for the scheduled UUID cleanup job
type UUIDCleanupConfig struct {
	Enabled             bool   `mapstructure:"enabled"`
	Schedule            string `mapstructure:"schedule"`
	InactivityDays      int    `mapstructure:"inactivity_days"`
	DeleteOrphanedUUIDs bool   `mapstructure:"delete_orphaned_uuids"`
	UseSoftDelete       bool   `mapstructure:"use_soft_delete"`
}

// Config holds the application's configuration, mapped from config.toml
type Config struct {
	Version         string // No tag needed, not from config file
	ConfigPath      string // No tag needed, internal use
	CheckForUpdates bool   `mapstructure:"check_for_updates"` // Changed tag from checkForUpdates
	SessionSecret   string `mapstructure:"session_secret"`    // Changed tag from sessionSecret

	Server      ServerConfig      `mapstructure:"server"`       // Nested Server config
	Database    DatabaseConfig    `mapstructure:"database"`     // Nested Database config
	Logging     LoggingConfig     `mapstructure:"logging"`      // Nested Logging config
	Valkey      ValkeyConfig      `mapstructure:"valkey"`       // Nested Valkey config
	RateLimit   RateLimitConfig   `mapstructure:"rate_limits"`   // Nested Rate Limit config
	UUIDCleanup UUIDCleanupConfig `mapstructure:"uuid_cleanup"` // Nested UUID Cleanup config
}

// ConfigUpdate struct remains for potential partial updates via API,
// but needs review if it's still used and how it maps to the nested structure.
// Keeping it as is for now as the primary task is fixing the config load.
type ConfigUpdate struct {
	Host            *string `json:"host,omitempty"`              // Maps to Server.Host
	Port            *int    `json:"port,omitempty"`              // Maps to Server.Port
	LogLevel        *string `json:"log_level,omitempty"`         // Maps to Logging.Level
	LogPath         *string `json:"log_path,omitempty"`          // Maps to Logging.Path
	BaseURL         *string `json:"base_url,omitempty"`          // Maps to Server.BaseURL
	CheckForUpdates *bool   `json:"check_for_updates,omitempty"` // Maps to Config.CheckForUpdates
}
