package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/pkg/errors"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var configTemplate = `# config.toml
 
 # Check for updates
 # Default is: true
 check_for_updates = true
 
 # Session secret
 # This is a randomly generated secret key for session management.
 # It will be generated automatically on the first run if not set.
 # Default: "{{ .sessionSecret }}" (dynamically generated)
 session_secret = "{{ .sessionSecret }}"
 
 [server]
   # Hostname or IP address for the server to listen on.
   # Default: "{{ .host }}" (e.g., "127.0.0.1" for local access, "0.0.0.0" for all interfaces, especially in Docker)
   host = "{{ .host }}"
 
   # Port for the server to listen on.
   # Default: 8282
   port = 8282
 
   # Base URL for serving the application under a subdirectory (e.g., /syncyomi/).
   # Leave empty if serving from the root or using a subdomain.
   # Optional.
   # Default: ""
   #base_url = ""
 
 [database]
   # Database type to use.
   # Supported: "sqlite", "postgres"
   # Optional.
   # Default: "sqlite"
   type = "sqlite" # Default type explicitly set here
 
   # --- PostgreSQL Settings ---
   # These settings are only used if database.type is set to "postgres".
   # Make sure PostgreSQL is installed and running before enabling.
   [database.postgres] # Corrected to ensure this is always present in template for viper to find keys
     # Hostname or IP address of the PostgreSQL server.
     # Optional.
     # Default: "localhost"
     host = "localhost"
 
     # Port of the PostgreSQL server.
     # Optional.
     # Default: 5432
     port = 5432
 
     # Name of the PostgreSQL database.
     # Optional.
     # Default: "postgres"
     database = "postgres"
 
     # Username for connecting to the PostgreSQL database.
     # Optional.
     # Default: "postgres"
     user = "postgres"
 
     # Password for the PostgreSQL user.
     # Optional.
     # Default: "postgres"
     pass = "postgres"
 
     # PostgreSQL SSL mode.
     # Options: "disable", "allow", "prefer", "require", "verify-ca", "verify-full"
     # Refer to PostgreSQL documentation for details: https://www.postgresql.org/docs/current/libpq-ssl.html#LIBPQ-SSL-SSLMODE-STATEMENTS
     # Optional.
     # Default: "disable"
     ssl_mode = "disable"
 
 [logging]
   # Log file path.
   # If empty or not set, logs will be written to standard output (stdout).
   # Use forward slashes for paths (e.g., "log/").
   # Optional.
   # Default: ""
   path = "log/"
 
   # Log level.
   # Determines the verbosity of logs.
   # Options: "ERROR", "WARN", "INFO", "DEBUG", "TRACE"
   # Default: "DEBUG"
   level = "DEBUG"
 
   # Maximum size of a log file in megabytes (MB) before it is rotated.
   # Default: 50
   max_file_size = 50
 
   # Maximum number of old log files to keep.
   # Default: 3
   max_backup_count = 3
 
 [valkey]
   # Valkey server address (e.g., "localhost:6379").
   # Optional.
   # Default: "localhost:6379"
   address = "localhost:6379"
 
   # Password for Valkey server.
   # Optional.
   # Default: "SyncYomi" (matches Docker configuration)
   password = "SyncYomi"
 
   # Valkey database number.
   # Optional.
   # Default: 0
   db = 0
   
 [rate_limit]
   # Enable rate limiting for profile-related endpoints
   # Default: true
   enabled = true
   
   # Maximum number of requests allowed per time window
   # Default: 20
   requests_per_minute = 20
   
   # Time window in seconds for rate limiting
   # Default: 60 (1 minute)
   window_seconds = 60
   
   # Comma-separated list of roles exempt from rate limiting
   # Default: "admin"
   exempt_roles = "admin"
   
   # Comma-separated list of internal IPs exempt from rate limiting
   # Default: "127.0.0.1,::1"
   exempt_internal_ips = "127.0.0.1,::1"
   
 [uuid_cleanup]
   # Enable scheduled cleanup of stale profile UUIDs
   # Default: true
   enabled = true
   
   # Cron schedule for the cleanup job
   # Default: "0 3 * * 0" (3 AM every Sunday)
   schedule = "0 3 * * 0"
   
   # Number of days of inactivity before a UUID is considered stale
   # Default: 365 (1 year)
   inactivity_days = 365
   
   # Whether to delete orphaned UUIDs (those without associated user records)
   # Default: true
   delete_orphaned_uuids = true
   
   # Whether to use soft delete (archive) instead of hard delete
   # Default: false (use hard delete)
   use_soft_delete = false
 `

var generateRandomString = func(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func writeConfig(configPath string, configFile string) error {
	cfgPath := filepath.Join(configPath, configFile)

	// check if configPath exists, if not create it
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(configPath, os.ModePerm)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	// check if config exists, if not create it
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		// set default host
		host := "127.0.0.1"

		if _, dockerErr := os.Stat("/.dockerenv"); dockerErr == nil {
			host = "0.0.0.0"
		} else if pd, cgroupErr := os.Open("/proc/1/cgroup"); cgroupErr == nil {
			defer func(pd *os.File) {
				errClose := pd.Close()
				if errClose != nil {
					log.Printf("error closing proc/cgroup: %q", errClose)
				}
			}(pd)
			b := make([]byte, 4096, 4096)
			_, readErr := pd.Read(b)
			if readErr != nil {
				// Log the error but don't necessarily fail config creation
				log.Printf("error reading /proc/1/cgroup: %v", readErr)
			} else {
				if strings.Contains(string(b), "/docker") || strings.Contains(string(b), "/lxc") {
					host = "0.0.0.0"
				}
			}
		}

		f, createErr := os.Create(cfgPath)
		if createErr != nil { // perm 0666
			log.Printf("error creating file: %q", createErr)
			return createErr
		}
		defer func(f *os.File) {
			errClose := f.Close()
			if errClose != nil {
				log.Printf("error closing file: %q", errClose)
			}
		}(f)

		sessionSecretVal, secretErr := generateRandomString(16) // Generate a 32-character hex string
		if secretErr != nil {
			log.Printf("Failed to generate session secret: %v. Using a default placeholder.", secretErr)
			sessionSecretVal = "fallback-please-replace-this-secret-immediately" // Fallback secret
		}

		tmpl, tmplErr := template.New("config").Parse(configTemplate)
		if tmplErr != nil {
			return errors.Wrap(tmplErr, "could not create config template")
		}

		tmplVars := map[string]string{
			"host":          host,
			"sessionSecret": sessionSecretVal,
		}

		var buffer bytes.Buffer
		if execErr := tmpl.Execute(&buffer, &tmplVars); execErr != nil {
			return errors.Wrap(execErr, "could not write config template output")
		}

		if _, writeErr := f.WriteString(buffer.String()); writeErr != nil {
			log.Printf("error writing contents to file: %v %q", configPath, writeErr)
			return writeErr
		}

		return f.Sync()
	}

	return nil
}

type Config interface {
	UpdateConfig() error
	DynamicReload(log logger.Logger)
}

type AppConfig struct {
	Config *domain.Config
	m      sync.Mutex
}

func New(configPath string, version string) *AppConfig {
	c := &AppConfig{}
	c.defaults() // Initialize with new nested structure
	c.Config.Version = version
	c.Config.ConfigPath = configPath

	c.load(configPath)

	return c
}

func (c *AppConfig) defaults() {
	c.Config = &domain.Config{
		Version:         "dev", // Internal, not from toml
		ConfigPath:      "",    // Internal, not from toml
		CheckForUpdates: true,
		SessionSecret:   "secret-session-key", // Will be overwritten by generated if not in file
		Server: domain.ServerConfig{
			Host:    "127.0.0.1",
			Port:    8282,
			BaseURL: "",
		},
		Database: domain.DatabaseConfig{
			Type: "sqlite", // Default database type
			Postgres: domain.PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "postgres",
				User:     "postgres",
				Pass:     "postgres",
				SslMode:  "disable",
			},
		},
		Logging: domain.LoggingConfig{
			Path:           "",
			Level:          "DEBUG",
			MaxFileSize:    50,
			MaxBackupCount: 3,
		},
		Valkey: domain.ValkeyConfig{
			Address:  "localhost:6379",
			Password: "SyncYomi",
			DB:       0,
		},
		RateLimit: domain.RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 20,
			WindowSeconds:     60,
			ExemptRoles:       "admin",
			ExemptInternalIPs: "127.0.0.1,::1",
		},
		UUIDCleanup: domain.UUIDCleanupConfig{
			Enabled:             true,
			Schedule:            "0 3 * * 0", // 3 AM every Sunday
			InactivityDays:      365,         // 1 year
			DeleteOrphanedUUIDs: true,
			UseSoftDelete:       false,
		},
	}
}

func (c *AppConfig) load(configPath string) {
	viper.SetConfigType("toml")
	configPath = path.Clean(configPath)

	if configPath != "" {
		if err := writeConfig(configPath, "config.toml"); err != nil {
			log.Printf("writeConfig error during load: %q", err)
			// Continue to attempt reading, defaults might be used or file might exist partially
		}
		viper.SetConfigFile(path.Join(configPath, "config.toml"))
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.config/syncyomi")
		viper.AddConfigPath("$HOME/.syncyomi")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Printf("Config file not found, using defaults: %s", viper.ConfigFileUsed())
		} else {
			log.Printf("Config read error: %q. Using defaults.", err)
		}
	}

	// Unmarshal the entire config structure
	if err := viper.Unmarshal(&c.Config); err != nil {
		log.Fatalf("Could not unmarshal config file into struct: %v. Config file used: %s", err, viper.ConfigFileUsed())
	}
}

func (c *AppConfig) DynamicReload(log logger.Logger) {
	viper.OnConfigChange(func(e fsnotify.Event) {
		c.m.Lock()
		defer c.m.Unlock()

		log.Info().Msgf("Config file changed: %s. Reloading configuration.", e.Name)

		// Re-read and unmarshal the entire config to capture all changes accurately
		if err := viper.ReadInConfig(); err != nil {
			log.Error().Err(err).Msg("Error reading config file during dynamic reload")
			return
		}

		var newConfig domain.Config
		// Preserve version and configPath as they are not from the file
		newConfig.Version = c.Config.Version
		newConfig.ConfigPath = c.Config.ConfigPath

		if err := viper.Unmarshal(&newConfig); err != nil {
			log.Error().Err(err).Msg("Error unmarshalling config during dynamic reload")
			return
		}

		// Atomically update the config
		c.Config = &newConfig

		// Update logger level if it changed
		log.SetLogLevel(c.Config.Logging.Level)

		log.Debug().Msg("Configuration reloaded successfully!")
	})
	viper.WatchConfig()
}

/*
// UpdateConfig and processLines were based on string manipulation and are NOT compatible
// with the nested TOML structure introduced. They would require a proper TOML parsing/modification
// library (e.g., github.com/BurntSushi/toml) to function correctly.
// Commenting them out to prevent interference with Viper's config reading/handling.
// The primary focus is ensuring Viper correctly loads the config on startup and reload.

func (c *AppConfig) UpdateConfig() error {
	file := path.Join(c.Config.ConfigPath, "config.toml")

	f, err := os.ReadFile(file)
	if err != nil {
		return errors.Wrap(err, "could not read config file: %s", file)
	}

	lines := strings.Split(string(f), "\n")
	lines = c.processLines(lines) // This will not work correctly with nested structures

	output := strings.Join(lines, "\n")
	if err := os.WriteFile(file, []byte(output), 0644); err != nil {
		return errors.Wrap(err, "could not write config file: %s", file)
	}

	return nil
}

func (c *AppConfig) processLines(lines []string) []string {
	// This function is not compatible with the new nested TOML structure
	// and will not correctly update values.
	log.Println("WARNING: processLines is not compatible with the new nested config structure and will not update values correctly.")

	var (
		foundLineUpdate   = false
		// For logging, we'd need to find [logging] then the keys
		// foundLineLogLevel = false
		// foundLineLogPath  = false
	)

	for i, line := range lines {
		// Example for a top-level key (still somewhat works)
		if !foundLineUpdate && strings.Contains(line, "check_for_updates =") {
			lines[i] = fmt.Sprintf("check_for_updates = %t", c.Config.CheckForUpdates)
			foundLineUpdate = true
		}
		// Updating nested keys like logging.level or server.host with this method is not feasible.
	}

	if !foundLineUpdate {
		lines = append(lines, fmt.Sprintf("check_for_updates = %t", c.Config.CheckForUpdates))
	}
	// Appending other missing keys would also need to respect TOML table structure.

	return lines
}
*/
