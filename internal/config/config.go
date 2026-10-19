package config

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/spf13/viper"
)

// isMissingConfigFile reports whether err means "no config file on disk",
// which is a supported way to run the service. Viper signals this either with
// ConfigFileNotFoundError (search-path miss) or an fs.PathError (named file
// absent), depending on how the config name was set.
func isMissingConfigFile(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	if errors.As(err, &notFound) {
		return true
	}
	var pathErr *fs.PathError
	return errors.As(err, &pathErr)
}

type Config struct {
	Server struct {
		Port string `mapstructure:"port"`
		Env  string `mapstructure:"env"`
	} `mapstructure:"server"`

	CORS struct {
		// Comma-separated list so the value works identically in YAML and in
		// a single env var (CORS_ALLOWED_ORIGINS). Use CORSOrigins() to read it.
		AllowedOrigins string `mapstructure:"allowed_origins"`
	} `mapstructure:"cors"`

	Auth struct {
		// Signs the short-lived HS256 API tokens the Next.js app mints after a
		// Google sign-in; RequireAuth verifies them. This is a dedicated secret,
		// deliberately not reused as the frontend's NEXTAUTH_SECRET, so rotating
		// one does not invalidate the other.
		JWTSecret string `mapstructure:"jwt_secret"`
	} `mapstructure:"auth"`

	Mongo struct {
		URI             string `mapstructure:"uri"`
		Database        string `mapstructure:"database"`
		AlertCollection string `mapstructure:"alert_collection"`
		SavedCollection string `mapstructure:"saved_collection"`
	} `mapstructure:"mongo"`

	Redis struct {
		Host            string `mapstructure:"host"`
		Port            string `mapstructure:"port"`
		Password        string `mapstructure:"password"`
		DB              int    `mapstructure:"db"`
		ViewTrackingTTL int    `mapstructure:"view_tracking_ttl"`
		KeyPrefix       string `mapstructure:"key_prefix"`
	} `mapstructure:"redis"`

	FX struct {
		APIURL          string `mapstructure:"api_url"`
		APIKEY          string `mapstructure:"api_key"`
		CacheTTLSeconds int    `mapstructure:"cache_ttl_seconds"`
	} `mapstructure:"fx"`

	OAuth struct {
		Google struct {
			ClientID     string `mapstructure:"client_id"`
			ClientSecret string `mapstructure:"client_secret"`
			RedirectURI  string `mapstructure:"redirect_uri"`
		} `mapstructure:"google"`

		Aliexpress struct {
			ClientID     string `mapstructure:"client_id"`
			ClientSecret string `mapstructure:"client_secret"`
			RedirectURI  string `mapstructure:"redirect_uri"`
		} `mapstructure:"aliexpress"`
	} `mapstructure:"oauth"`
}

// CORSOrigins splits the configured allowlist into individual origins.
// An empty config yields no origins, which the CORS middleware treats as
// "reject all cross-origin requests" rather than "allow all".
func (c *Config) CORSOrigins() []string {
	raw := strings.Split(c.CORS.AllowedOrigins, ",")
	out := make([]string, 0, len(raw))
	for _, o := range raw {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// IsProduction reports whether the service is running in production mode.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(c.Server.Env), "production")
}

// setDefaults registers every key with viper. This serves two purposes:
// it makes the service bootable with no config file at all, and it lets
// AutomaticEnv resolve nested keys (viper only consults the environment for
// keys it already knows about).
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.env", "development")

	v.SetDefault("cors.allowed_origins", "http://localhost:3000")

	v.SetDefault("auth.jwt_secret", "")

	v.SetDefault("mongo.uri", "mongodb://localhost:27017")
	v.SetDefault("mongo.database", "shopally")
	v.SetDefault("mongo.alert_collection", "alerts")
	v.SetDefault("mongo.saved_collection", "saved_items")

	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", "6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.view_tracking_ttl", 86400)
	v.SetDefault("redis.key_prefix", "sa:")

	v.SetDefault("fx.api_url", "")
	v.SetDefault("fx.api_key", "")
	v.SetDefault("fx.cache_ttl_seconds", 3600)

	v.SetDefault("oauth.google.client_id", "")
	v.SetDefault("oauth.google.client_secret", "")
	v.SetDefault("oauth.google.redirect_uri", "")
	v.SetDefault("oauth.aliexpress.client_id", "")
	v.SetDefault("oauth.aliexpress.client_secret", "")
	v.SetDefault("oauth.aliexpress.redirect_uri", "")
}

// LoadConfig reads configs/config.dev.yaml relative to path when present, then
// overlays environment variables. Nested keys map to underscored, upper-case
// names: mongo.uri -> MONGO_URI, auth.jwt_secret -> AUTH_JWT_SECRET.
//
// A missing config file is not an error. Hosted environments (Render, Fly,
// Cloud Run) inject configuration as env vars, and defaults cover local dev.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	setDefaults(v)

	v.SetConfigName("configs/config.dev")
	v.SetConfigType("yaml")
	v.AddConfigPath(path)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil && !isMissingConfigFile(err) {
		// Only a genuinely unreadable or malformed file is fatal.
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
