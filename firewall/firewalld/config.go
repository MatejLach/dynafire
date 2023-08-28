package firewalld

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	LogLevel         string `json:"log_level"`
	ZoneTargetPolicy string `json:"zone_target_policy"`
}

func initConfig() {
	slog.Info("No config.json found, creating new config...")
	config := Config{
		LogLevel:         "INFO",
		ZoneTargetPolicy: "ACCEPT",
	}

	jsonBytes, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		slog.Error("unable to parse config file", "details", err)
		os.Exit(1)
	}

	err = os.MkdirAll("/etc/dynafire", 0775)
	if err != nil {
		slog.Error("unable to make config directory", "details", err)
		os.Exit(1)
	}

	err = os.WriteFile("/etc/dynafire/config.json", jsonBytes, 0775)
	if err != nil {
		slog.Error("unable to save default config file", "details", err)
		os.Exit(1)
	}

	slog.Info("New config.json created, feel free to modify the defaults, then restart for your changes to take effect.")
}

func parseConfig() (Config, error) {
	var config Config
	cfgData, err := os.ReadFile("/etc/dynafire/config.json")
	if err != nil {
		return Config{}, err
	}

	err = json.Unmarshal(cfgData, &config)
	if err != nil {
		return Config{}, err
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parseLogLevel(config.LogLevel)})))

	return config, nil
}

func configExists() bool {
	if _, err := os.Stat("/etc/dynafire/config.json"); os.IsNotExist(err) {
		return false
	}

	return true
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
