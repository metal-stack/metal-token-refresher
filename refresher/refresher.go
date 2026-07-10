package refresher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apiclient "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/metal-token-refresher/spec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type (
	ClientForToken func(token string) (apiclient.Client, error)

	Refresher struct {
		log            *slog.Logger
		clientset      kubernetes.Interface
		clientForToken ClientForToken
	}

	TokenSecretKeyRef struct {
		Namespace string
		Name      string
		Key       string
	}
)

func New(log *slog.Logger, clientset kubernetes.Interface, clientForToken ClientForToken) *Refresher {
	return &Refresher{
		log:            log,
		clientset:      clientset,
		clientForToken: clientForToken,
	}
}

func (r *Refresher) RefreshSecret(ctx context.Context, ref TokenSecretKeyRef) error {
	r.log.With("namespace", ref.Namespace, "name", ref.Name)

	tokSec, err := r.clientset.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		r.log.Error("failed to fetch token secret", "error", err)
		return err
	}

	tok, ok := tokSec.Data[ref.Key]
	if !ok {
		r.log.Error("missing token in secret", "key", ref.Key)
		return fmt.Errorf("key %q not found in secret %s/%s", ref.Key, ref.Namespace, ref.Name)
	}

	client, err := r.clientForToken(string(tok))
	if err != nil {
		r.log.Error("failed to create a metal client for token", "error", err)
		return err
	}

	r.log.Info("refreshing token...")

	tokResp, err := client.Apiv2().Token().Refresh(ctx, &apiv2.TokenServiceRefreshRequest{})
	if err != nil {
		r.log.Error("failed to refresh token", "error", err)
		return err
	}

	tokSec.Data[ref.Key] = []byte(tokResp.Secret)

	annos := tokSec.Annotations
	if annos == nil {
		annos = map[string]string{}
	}

	annos[spec.AnnotationTokenUser] = tokResp.Token.User
	annos[spec.AnnotationTokenDescription] = tokResp.Token.Description
	annos[spec.AnnotationTokenExpires] = tokResp.Token.Expires.AsTime().Format(time.RFC3339)
	annos[spec.AnnotationTokenIssuedAt] = tokResp.Token.IssuedAt.AsTime().Format(time.RFC3339)

	tokSec.SetAnnotations(annos)

	_, err = r.clientset.CoreV1().Secrets(ref.Namespace).Update(ctx, tokSec, metav1.UpdateOptions{})
	if err != nil {
		r.log.Error("failed to update token secret", "error", err)
		return err
	}

	r.log.Info("updated token secret")

	return nil
}
