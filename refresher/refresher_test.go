package refresher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	apiclient "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	apitests "github.com/metal-stack/api/go/tests"
	"github.com/metal-stack/metal-token-refresher/spec"
	"github.com/stretchr/testify/mock"
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
	tests := []struct {
		name         string
		ref          TokenSecretKeyRef
		responseOk   *apiv2.TokenServiceRefreshResponse
		responseErr  error
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
					"token": []byte("old-token"),
				},
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: "new-token",
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						spec.AnnotationTokenUser:        "some-user",
						spec.AnnotationTokenDescription: "some description",
						spec.AnnotationTokenExpires:     "2006-01-02T16:04:05Z",
						spec.AnnotationTokenIssuedAt:    "2006-01-02T14:04:05Z",
					},
				},
				Data: map[string][]byte{
					"token": []byte("new-token"),
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
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						spec.AnnotationTokenUser:        "outdated-user",
						spec.AnnotationTokenDescription: "outdated description",
						spec.AnnotationTokenExpires:     "2006-01-01T16:04:05Z",
						spec.AnnotationTokenIssuedAt:    "2006-01-01T14:04:05Z",
						"another-annotation":            "keep-it",
					},
				},
				Data: map[string][]byte{
					"token": []byte("old-token"),
				},
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: "new-token",
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						spec.AnnotationTokenUser:        "some-user",
						spec.AnnotationTokenDescription: "some description",
						spec.AnnotationTokenExpires:     "2006-01-02T16:04:05Z",
						spec.AnnotationTokenIssuedAt:    "2006-01-02T14:04:05Z",
						"another-annotation":            "keep-it",
					},
				},
				Data: map[string][]byte{
					"token": []byte("new-token"),
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
					"token":     []byte("old-token"),
					"something": []byte("keep it"),
				},
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: "new-token",
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						spec.AnnotationTokenUser:        "some-user",
						spec.AnnotationTokenDescription: "some description",
						spec.AnnotationTokenExpires:     "2006-01-02T16:04:05Z",
						spec.AnnotationTokenIssuedAt:    "2006-01-02T14:04:05Z",
					},
				},
				Data: map[string][]byte{
					"token":     []byte("new-token"),
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
					"secret-token": []byte("old-token"),
				},
			},
			responseOk: &apiv2.TokenServiceRefreshResponse{
				Secret: "new-token",
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						spec.AnnotationTokenUser:        "some-user",
						spec.AnnotationTokenDescription: "some description",
						spec.AnnotationTokenExpires:     "2006-01-02T16:04:05Z",
						spec.AnnotationTokenIssuedAt:    "2006-01-02T14:04:05Z",
					},
				},
				Data: map[string][]byte{
					"secret-token": []byte("new-token"),
				},
			},
		},
		// error cases
		{
			name: "fails when refresh fails",
			ref: TokenSecretKeyRef{
				Name:      "some-secret",
				Namespace: "my-namespace",
				Key:       "token",
			},
			beforeSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte("old-token"),
				},
			},
			responseErr: fmt.Errorf("internal server error"),
			wantSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte("old-token"),
				},
			},
			wantError: "internal server error",
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
				Secret: "new-token",
				Token: &apiv2.Token{
					User:        "some-user",
					Description: "some description",
					Expires:     mustTimestamp("2006-01-02T16:04:05Z"),
					IssuedAt:    mustTimestamp("2006-01-02T14:04:05Z"),
				},
			},
			wantSecret: &v1.Secret{
				Data: map[string][]byte{
					"token": []byte("old-token"),
				},
			},
			wantError: "secrets \"some-secret\" not found",
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
				mocks := apitests.New(t)
				return mocks.Client(&apitests.ClientMockFns{
					Apiv2Mocks: &apitests.Apiv2MockFns{
						Token: func(m *mock.Mock) {
							m.On("Refresh", mock.IsType(context.Background()), &apiv2.TokenServiceRefreshRequest{}).
								Return(tt.responseOk, tt.responseErr)
						},
					},
				}), nil
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
