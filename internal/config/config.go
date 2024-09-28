package config

import "github.com/spf13/viper"

type Config struct {
	OKX struct {
		ApiKey     string `mapstructure:"api_key"`
		SecretKey  string `mapstructure:"secret_key"`
		Passphrase string `mapstructure:"passphrase"`
	} `mapstructure:"okx"`
}

func New(folder, filename string) (*Config, error) {
	cfg := new(Config)

	viper.AddConfigPath(folder)
	viper.SetConfigName(filename)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
