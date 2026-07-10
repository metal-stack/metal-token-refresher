package main

import (
	"context"
	"log/slog"
	"os"

	apiclient "github.com/metal-stack/api/go/client"
	"github.com/metal-stack/metal-token-refresher/refresher"
	"github.com/metal-stack/metal-token-refresher/spec"
	"github.com/metal-stack/v"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})
	log := slog.New(jsonHandler)

	log.Info("starting metal-token-refresher",
		"version", v.Version,
		"revision", v.Revision,
		"git-sha1", v.GitSHA1,
		"build-date", v.BuildDate,
	)

	cfg, err := spec.LoadConfig()
	if err != nil {
		log.Error("configuration error", "error", err)
		panic(err)
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		log.Error("failed to fetch in cluster rest config", "error", err)
		panic(err)
	}
	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		log.Error("failed to create in cluster client", "error", err)
		panic(err)
	}

	log.Info("refreshing metal-apiserver token", "metal-apiserver-url", cfg.MetalAPIServerURL)

	refresh := refresher.New(log, cs, func(token string) (apiclient.Client, error) {
		dial := &apiclient.DialConfig{
			BaseURL: cfg.MetalAPIServerURL,
			Log:     log,
			Token:   token,
		}

		return apiclient.New(dial)
	})

	err = refresh.RefreshSecret(context.Background(), refresher.TokenSecretKeyRef{
		Namespace: cfg.SecretNamespace,
		Name:      cfg.SecretName,
		Key:       cfg.SecretKey,
	})
	if err != nil {
		panic(err)
	}
}
