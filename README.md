# metal-token-refresher

Refreshes metal-apiserver-tokens stored in Kubernetes secrets as a CronJob.

## Configuration

To configure the metal-token-refresher to refresh a secret, set the following environment variables accordingly.

- `METAL_APISERVER_URL`
- `TOKEN_SECRET_NAMESPACE`
- `TOKEN_SECRET_NAME`
- `TOKEN_SECRET_KEY`

```yaml
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: metal-token-refresher
spec:
  schedule: "@hourly"
  jobTemplate:
    metadata:
      name: metal-token-refresher
    spec:
      template:
        spec:
          serviceAccountName: metal-token-refresher
          restartPolicy: OnFailure
          containers:
          - name: token-refresher
            image: ghcr.io/metal-stack/metal-token-refresher
            env:
            - name: METAL_APISERVER_URL
              value: http://metal-apiserver:8080
            - name: TOKEN_SECRET_NAMESPACE
              value: "metal-control-plane"
            - name: TOKEN_SECRET_NAME
              value: "token-secret"
            - name: TOKEN_SECRET_KEY
              value: "token"
---
apiVersion: v1
kind: Secret
metadata:
  name: token-secret
data:
  token: "..."
```

Also make sure to configure the service account accordingly.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: metal-token-renewal
  namespace: metal-stack
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: metal-console-token
  namespace: metal-stack
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "update", "patch"]
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["token-secret"]
  verbs: ["get", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: metal-token-renewal
  namespace: metal-stack
subjects:
- kind: ServiceAccount
  name: metal-token-renewal
  namespace: metal-stack
roleRef:
  kind: Role
  name: metal-token-renewal
  apiGroup: rbac.authorization.k8s.io
```
