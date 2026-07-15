package spec

import (
	"errors"
	"fmt"
	"os"
)

type Config struct {
	MetalAPIServerURL string
	SecretNamespace   string
	SecretName        string
	SecretKey         string
}

func LoadConfig() (*Config, error) {
	var (
		cfg  = &Config{}
		errs []error
		ok   bool
	)

	cfg.MetalAPIServerURL, ok = os.LookupEnv("METAL_APISERVER_URL")
	if !ok {
		errs = append(errs, fmt.Errorf("missing required METAL_APISERVER_URL"))
	}

	cfg.SecretNamespace, ok = os.LookupEnv("TOKEN_SECRET_NAMESPACE")
	if !ok {
		errs = append(errs, fmt.Errorf("missing required TOKEN_SECRET_NAMESPACE"))
	}

	cfg.SecretName, ok = os.LookupEnv("TOKEN_SECRET_NAME")
	if !ok {
		errs = append(errs, fmt.Errorf("missing required TOKEN_SECRET_NAME"))
	}

	cfg.SecretKey, ok = os.LookupEnv("TOKEN_SECRET_KEY")
	if !ok {
		cfg.SecretKey = "token"
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return cfg, nil
}
