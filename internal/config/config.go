package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Username  string `json:"username"`
	AuthKey   string `json:"auth_key,omitempty"`
	ThemeName string `json:"theme"`
	MicID     string `json:"mic_id,omitempty"`
	SpeakerID string `json:"speaker_id,omitempty"`
	WebcamID  string `json:"webcam_id,omitempty"`
}

func configPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "moistchat", "config.json")
}

func Exists() bool {
	_, err := os.Stat(configPath())
	return err == nil
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return &Config{}, nil
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return &Config{}, nil
	}
	return &c, nil
}

func (c *Config) Save() error {
	configDir := filepath.Dir(configPath())
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(configPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func SaveUsername(name string) error {
	cfg, _ := Load()
	cfg.Username = name
	return cfg.Save()
}

func SaveAuthKey(key string) error {
	cfg, _ := Load()
	cfg.AuthKey = key
	return cfg.Save()
}

func SaveThemeName(name string) error {
	cfg, _ := Load()
	cfg.ThemeName = name
	return cfg.Save()
}

func SaveMicID(id string) error {
	cfg, _ := Load()
	cfg.MicID = id
	return cfg.Save()
}

func SaveSpeakerID(id string) error {
	cfg, _ := Load()
	cfg.SpeakerID = id
	return cfg.Save()
}

func SaveWebcamID(id string) error {
	cfg, _ := Load()
	cfg.WebcamID = id
	return cfg.Save()
}
