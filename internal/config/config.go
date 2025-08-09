package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Alias struct {
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Secure    bool   `yaml:"secure"`
	PathStyle bool   `yaml:"path_style"`
}

type Config struct {
	Aliases map[string]Alias `yaml:"aliases"`
}

var (
	ErrAliasExists   = errors.New("алиас уже существует")
	ErrAliasNotFound = errors.New("алиас не найден")
	ErrInvalidAlias  = errors.New("некорректное имя алиаса")
)

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить домашний каталог: %w", err)
	}
	return filepath.Join(home, ".s3cli", "config.yaml"), nil
}

func Load(path string) (*Config, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Aliases: map[string]Alias{}}, nil
		}
		return nil, fmt.Errorf("не удалось прочитать конфиг %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("повреждён конфиг %q: %w", path, err)
	}
	if cfg.Aliases == nil {
		cfg.Aliases = map[string]Alias{}
	}
	return &cfg, nil
}

func Save(path string, cfg *Config) error {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("не удалось создать каталог %q: %w", dir, err)
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("ошибка сериализации конфига: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("не удалось записать %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("не удалось заменить %q: %w", path, err)
	}
	return nil
}

func (c *Config) AddAlias(name string, a Alias) error {
	if name == "" {
		return ErrInvalidAlias
	}
	if _, ok := c.Aliases[name]; ok {
		return ErrAliasExists
	}
	c.Aliases[name] = a
	return nil
}

func (c *Config) RemoveAlias(name string) error {
	if _, ok := c.Aliases[name]; !ok {
		return ErrAliasNotFound
	}
	delete(c.Aliases, name)
	return nil
}

func (c *Config) GetAlias(name string) (Alias, error) {
	a, ok := c.Aliases[name]
	if !ok {
		return Alias{}, ErrAliasNotFound
	}
	return a, nil
}

func (c *Config) List() map[string]Alias {
	out := make(map[string]Alias, len(c.Aliases))
	for k, v := range c.Aliases {
		out[k] = v
	}
	return out
}
