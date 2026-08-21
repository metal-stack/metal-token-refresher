package refresher

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/golang-jwt/jwt/v5"
	apiclient "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/metal-token-refresher/spec"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fake "k8s.io/client-go/kubernetes/fake"
)

func mustTimestamp(date string) *timestamppb.Timestamp {
	parsed, err := time.Parse(time.RFC3339, date)
	if err != nil {
		panic(fmt.Errorf("failed to parse date %q", err))
	}

	return timestamppb.New(parsed)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func TestRefreshSecret(t *testing.T) {
	var (
		oldToken string
		newToken string
	)

	oldToken, err := generateToken(10 * time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token, due to %s", err)
	}

	newToken, err = generateToken(10 * time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token, due to %s", err)
	}

	tests := []struct {
		name         string
		ref          TokenSecretKeyRef
		responseOk   *apiv2.TokenServiceRefreshResponse
		responseErr  *connect.Error
		beforeSecret *v1.Secret
		wantSecret   *v1.Secret
		wantError    string
	}{
		{
			name: "refreshes token and patches existing one",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte(oldToken),
				},
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: newToken,
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				Annotations: map[string]string{
					spec.AnnotationTokenUser:        "some-user",
					spec.AnnotationTokenDescription: "some description",
					spec.AnnotationTokenExpires:     "2006-01-02T16:04:05Z",
					spec.AnnotationTokenIssuedAt:    "2006-01-02T14:04:05Z",
				},
				Data: map[string][]byte{
					"token": []byte(newToken),
				},
			},
		},
		{
			name: "overrides only conflicting annotations",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: &v1.Secret{
				Annotations: map[string]string{
					spec.AnnotationTokenUser:        "outdated-user",
					spec.AnnotationTokenDescription: "outdated description",
					spec.AnnotationTokenExpires:     "2006-01-01T16:04:05Z",
					spec.AnnotationTokenIssuedAt:    "2006-01-01T14:04:05Z",
					"another-annotation":            "keep-it",
				},
				Data: map[string][]byte{
					"token": []byte(oldToken),
				},
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: newToken,
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				Annotations: map[string]string{
					spec.AnnotationTokenUser:        "some-user",
					spec.AnnotationTokenDescription: "some description",
					spec.AnnotationTokenExpires:     "2006-01-02T16:04:05Z",
					spec.AnnotationTokenIssuedAt:    "2006-01-02T14:04:05Z",
					"another-annotation":            "keep-it",
				},
				Data: map[string][]byte{
					"token": []byte(newToken),
				},
			},
		},
		{
			name: "keeps other data as is",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: &v1.Secret{
				Data: map[string][]byte{
					"token":     []byte(oldToken),
					"something": []byte("keep it"),
				},
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: newToken,
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				Annotations: map[string]string{
					spec.AnnotationTokenUser:        "some-user",
					spec.AnnotationTokenDescription: "some description",
					spec.AnnotationTokenExpires:     "2006-01-02T16:04:05Z",
					spec.AnnotationTokenIssuedAt:    "2006-01-02T14:04:05Z",
				},
				Data: map[string][]byte{
					"token":     []byte(newToken),
					"something": []byte("keep it"),
				},
			},
		},
		{
			name: "respects token key",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "secret-token",
			},
			beforeSecret: &v1.Secret{
				Data: map[string][]byte{
					"secret-token": []byte(oldToken),
				},
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: newToken,
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				Annotations: map[string]string{
					spec.AnnotationTokenUser:        "some-user",
					spec.AnnotationTokenDescription: "some description",
					spec.AnnotationTokenExpires:     "2006-01-02T16:04:05Z",
					spec.AnnotationTokenIssuedAt:    "2006-01-02T14:04:05Z",
				},
				Data: map[string][]byte{
					"secret-token": []byte(newToken),
				},
			},
		},
		// error cases
		{
			name: "fails when secret is empty",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: &v1.Secret{
				Data: nil,
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: newToken,
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				Data: nil,
			},
			wantError: `key "token" not found in secret my-namespace/some-secret`,
		},
		{
			name: "fails when refresh fails due to some api error",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte(oldToken),
				},
			},
			responseErr: connect.NewError(connect.CodeInternal, errors.New("internal server error")),
			wantSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte(oldToken),
				},
			},
			wantError: "internal: internal server error",
		},
		{
			name: "fails when refresh fails due to unauthenticated",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte(oldToken),
				},
			},
			responseErr: connect.NewError(connect.CodeUnauthenticated, errors.New("token expired")),
			wantSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte(oldToken),
				},
			},
			wantError: "unauthenticated: token expired",
		},
		{
			name: "fails when secret not found",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: nil,
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: newToken,
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte(oldToken),
				},
			},
			wantError: `secrets "some-secret" not found`,
		},
		{
			name: "fails when token in secret is invalid",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte("invalid token"),
				},
			},
			wantSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte("invalid token"),
				},
			},
			// this error is thrown on apiclient creation
			wantError: `unable to parse token:token is malformed: token contains an invalid number of segments`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			objs := []runtime.Object{}
			if tt.beforeSecret != nil {
				tt.beforeSecret.Name = tt.ref.Name
				tt.beforeSecret.Namespace = tt.ref.Namespace
				objs = append(objs, tt.beforeSecret)
			}
			cs := fake.NewSimpleClientset(objs...)

			ref := New(testLogger(), cs, func(token string) (apiclient.Client, error) {
				refreshCall := apiclient.ClientCall{
					WantRequest: &apiv2.TokenServiceRefreshRequest{},
				}

				if tt.responseOk != nil {
					refreshCall.WantResponse = func() connect.AnyResponse {
						return connect.NewResponse(tt.responseOk)
					}
				} else if tt.responseErr != nil {
					refreshCall.WantError = tt.responseErr
				}

				return apiclient.New(&apiclient.DialConfig{
					BaseURL: "http://localhost",
					Token:   token,
					Interceptors: []connect.Interceptor{
						apiclient.NewTestInterceptor(t, []apiclient.ClientCall{
							refreshCall,
						}),
					},
				})
			})

			err := ref.RefreshSecret(context.Background(), tt.ref)

			if (tt.wantError == "") != (err == nil) {
				t.Errorf("want error %q, got %q", tt.wantError, err)
			} else if err != nil && err.Error() != tt.wantError {
				t.Errorf("want error %q, got %q", tt.wantError, err)
			}

			if tt.wantError != "" {
				return
			}

			gotSecret, err := cs.CoreV1().Secrets(tt.ref.Namespace).Get(context.Background(), tt.ref.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get updated secret: %v", err)
			}

			if tt.wantSecret != nil {
				for k, wantv := range tt.wantSecret.Annotations {
					gotv := gotSecret.Annotations[k]
					if gotv != wantv {
						t.Errorf("want annotation %q=%q, got value %q", k, wantv, gotv)
					}
				}

				for k, wantv := range tt.wantSecret.Data {
					gotv := gotSecret.Data[k]
					if string(gotv) != string(wantv) {
						t.Errorf("want data %q=%q, got %q", k, wantv, gotv)
					}
				}
			}
		})
	}
}

func generateToken(duration time.Duration) (string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}

	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
		Issuer:    "test",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
