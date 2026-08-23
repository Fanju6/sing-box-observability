package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Console   ConsoleConfig   `yaml:"console"`
	Singbox   SingboxConfig   `yaml:"singbox"`
	Collector CollectorConfig `yaml:"collector"`
	Storage   StorageConfig   `yaml:"storage"`
}

type ServerConfig struct {
	Listen         string   `yaml:"listen"`
	TrustedProxies []string `yaml:"trusted_proxies"`
	SecureCookie   bool     `yaml:"secure_cookie"`
}

type ConsoleConfig struct {
	AuthToken  string        `yaml:"auth_token"`
	SessionTTL time.Duration `yaml:"session_ttl"`
}

type SingboxConfig struct {
	Name    string `yaml:"name"`
	BaseURL string `yaml:"base_url"`
	Token   string `yaml:"token"`
}

type CollectorConfig struct {
	ScrapeInterval    time.Duration `yaml:"scrape_interval"`
	PersistInterval   time.Duration `yaml:"persist_interval"`
	ReconcileInterval time.Duration `yaml:"reconcile_interval"`
	StaleAfter        time.Duration `yaml:"stale_after"`
}

type StorageConfig struct {
	Path      string        `yaml:"path"`
	Retention time.Duration `yaml:"retention"`
}

func Default() Config {
	return Config{
		Server:    ServerConfig{Listen: "127.0.0.1:9095"},
		Console:   ConsoleConfig{SessionTTL: 24 * time.Hour},
		Singbox:   SingboxConfig{Name: "local", BaseURL: "http://127.0.0.1:9090"},
		Collector: CollectorConfig{ScrapeInterval: 2 * time.Second, PersistInterval: 15 * time.Second, ReconcileInterval: 3 * time.Second, StaleAfter: 30 * time.Second},
		Storage:   StorageConfig{Path: "./data/observability.db", Retention: 7 * 24 * time.Hour},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("decode config: %w", err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			if err != nil {
				return Config{}, fmt.Errorf("decode trailing config: %w", err)
			}
			return Config{}, errors.New("decode config: multiple YAML documents are not allowed")
		}
	}
	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}
	if err := Validate(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnv(c *Config) error {
	if v := os.Getenv("SBOBS_SERVER_LISTEN"); v != "" {
		c.Server.Listen = v
	}
	if v := os.Getenv("SBOBS_SERVER_SECURE_COOKIE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("SBOBS_SERVER_SECURE_COOKIE: %w", err)
		}
		c.Server.SecureCookie = b
	}
	if v, ok := os.LookupEnv("SBOBS_CONSOLE_AUTH_TOKEN"); ok {
		c.Console.AuthToken = v
	}
	if v := os.Getenv("SBOBS_CONSOLE_SESSION_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("SBOBS_CONSOLE_SESSION_TTL: %w", err)
		}
		c.Console.SessionTTL = d
	}
	if v := os.Getenv("SBOBS_SINGBOX_NAME"); v != "" {
		c.Singbox.Name = v
	}
	if v := os.Getenv("SBOBS_SINGBOX_BASE_URL"); v != "" {
		c.Singbox.BaseURL = v
	}
	if v, ok := os.LookupEnv("SBOBS_SINGBOX_TOKEN"); ok {
		c.Singbox.Token = v
	}
	for _, item := range []struct {
		name string
		dst  *time.Duration
	}{
		{"SBOBS_COLLECTOR_SCRAPE_INTERVAL", &c.Collector.ScrapeInterval},
		{"SBOBS_COLLECTOR_PERSIST_INTERVAL", &c.Collector.PersistInterval},
		{"SBOBS_COLLECTOR_RECONCILE_INTERVAL", &c.Collector.ReconcileInterval},
		{"SBOBS_COLLECTOR_STALE_AFTER", &c.Collector.StaleAfter},
		{"SBOBS_STORAGE_RETENTION", &c.Storage.Retention},
	} {
		if v := os.Getenv(item.name); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("%s: %w", item.name, err)
			}
			*item.dst = d
		}
	}
	if v := os.Getenv("SBOBS_STORAGE_PATH"); v != "" {
		c.Storage.Path = v
	}
	if v := os.Getenv("SBOBS_SERVER_TRUSTED_PROXIES"); v != "" {
		c.Server.TrustedProxies = splitList(v)
	}
	return nil
}

func splitList(value string) []string {
	var out []string
	for _, v := range strings.Split(value, ",") {
		if strings.TrimSpace(v) != "" {
			out = append(out, strings.TrimSpace(v))
		}
	}
	return out
}

func Validate(c *Config) error {
	if c.Server.Listen == "" {
		return errors.New("server.listen is required")
	}
	if c.Console.SessionTTL <= 0 {
		return errors.New("console.session_ttl must be positive")
	}
	u, err := url.Parse(c.Singbox.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("singbox.base_url must be an http/https URL without userinfo or fragment")
	}
	if c.Collector.ScrapeInterval <= 0 || c.Collector.PersistInterval <= 0 || c.Collector.ReconcileInterval <= 0 || c.Collector.StaleAfter <= 0 {
		return errors.New("collector intervals must be positive")
	}
	if c.Collector.PersistInterval < c.Collector.ScrapeInterval {
		return errors.New("collector.persist_interval must be at least scrape_interval")
	}
	if c.Storage.Retention < time.Hour {
		return errors.New("storage.retention must be at least 1h")
	}
	if c.Storage.Path == "" {
		return errors.New("storage.path is required")
	}
	if c.Console.AuthToken == "" && !isLoopbackListen(c.Server.Listen) {
		return errors.New("console.auth_token is required for non-loopback listening / 非回环监听必须启用控制台认证")
	}
	return nil
}

func isLoopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if host == "localhost" || host == "" {
		return host == "localhost"
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func IsTrustedProxy(c Config, remoteIP string) bool {
	for _, proxy := range c.Server.TrustedProxies {
		if proxy == remoteIP {
			return true
		}
	}
	return false
}
