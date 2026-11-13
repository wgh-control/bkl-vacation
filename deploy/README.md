# Deploy

GitLab CI builds one image and pushes it tagged with the commit SHA. Argo CD syncs this directory onto Kubernetes. The pipeline does not apply manifests.

PostgreSQL, Keycloak, SMTP, and Vault stay outside the cluster. This tree does not install them, and it does not install External Secrets Operator or Argo CD. Those operators are platform prerequisites.

Runtime secrets are read from HashiCorp Vault by External Secrets Operator, which writes Secret `bkl-vacation`. The API, the worker, and the migration Job consume that Secret as environment variables. No secret values are stored in git.

Until Vault is connected, set the same secret names as masked GitLab CI/CD variables. The pipeline does not read the runtime ones, and it does not copy them into the cluster. Copy them into the Vault path below, then remove the runtime variables from GitLab.

## Variables

Secrets belong in Vault. Until that path is connected, the GitLab CI/CD variable of the same name is the place to fill the value. Non-secrets stay in GitLab or in the committed ConfigMap, as the Where column says.

| Name | Purpose | Where it is set |
| --- | --- | --- |
| `DATABASE_URL` | External Postgres DSN, with TLS | Vault KV v2 `placeholder-vault-mount`, path `bkl-vacation/runtime`, property `DATABASE_URL`. Until Vault is connected: masked GitLab CI/CD variable `DATABASE_URL` |
| `KEYCLOAK_CLIENT_SECRET` | Client secret of the portal client in the existing Keycloak realm | Vault `placeholder-vault-mount` / `bkl-vacation/runtime` property `KEYCLOAK_CLIENT_SECRET`. Until Vault is connected: masked GitLab CI/CD variable `KEYCLOAK_CLIENT_SECRET` |
| `SMTP_USERNAME` | Username for the existing SMTP server | Vault `placeholder-vault-mount` / `bkl-vacation/runtime` property `SMTP_USERNAME`. Until Vault is connected: masked GitLab CI/CD variable `SMTP_USERNAME` |
| `SMTP_PASSWORD` | Password for the existing SMTP server | Vault `placeholder-vault-mount` / `bkl-vacation/runtime` property `SMTP_PASSWORD`. Until Vault is connected: masked GitLab CI/CD variable `SMTP_PASSWORD` |
| `REGISTRY_PASSWORD` | Password used to push the image | Vault `placeholder-vault-mount` / `bkl-vacation/ci` property `REGISTRY_PASSWORD` once GitLab is connected to Vault. Until then: masked GitLab CI/CD variable `REGISTRY_PASSWORD`. The pipeline reads the GitLab variable |
| `REGISTRY_USER` | User used to push the image | Vault `placeholder-vault-mount` / `bkl-vacation/ci` property `REGISTRY_USER` once GitLab is connected to Vault. Until then: masked GitLab CI/CD variable `REGISTRY_USER`. The pipeline reads the GitLab variable |
| `GIT_PUSH_TOKEN` | Token that can push the image-tag commit to the git revision Argo CD watches | Vault `placeholder-vault-mount` / `bkl-vacation/ci` property `GIT_PUSH_TOKEN` once GitLab is connected to Vault. Until then: masked GitLab CI/CD variable `GIT_PUSH_TOKEN`. The pipeline reads the GitLab variable |
| `REGISTRY_HOST` | Container registry host. Replaces `placeholder.invalid` in the image name | GitLab CI/CD variable `REGISTRY_HOST`. Not a secret |
| `IMAGE_NAME` | Image repository path under that host. Replaces `placeholder-image-name` | GitLab CI/CD variable `IMAGE_NAME`. Not a secret |
| `GIT_PUSH_URL` | HTTPS URL of the git revision Argo CD watches, with no credentials in the URL. With the recommended pull mirror this is `https://github.com/wgh-control/bkl-vacation.git` | GitLab CI/CD variable `GIT_PUSH_URL`. Not a secret |
| `GIT_PUSH_USER` | Username for `GIT_PUSH_TOKEN`. Default `oauth2`. For GitHub set `x-access-token` | GitLab CI/CD variable `GIT_PUSH_USER`. Not a secret |
| `GO_VERSION` | Go toolchain for test and image build. Default `1.25`. Match the `go` line in `go.mod` | GitLab CI/CD variable `GO_VERSION`. Optional |
| `NODE_VERSION` | Node toolchain for frontend test and the SPA build. Default `22` | GitLab CI/CD variable `NODE_VERSION`. Optional |
| `SPA_DIST` | Frontend build output directory. Default `dist` | GitLab CI/CD variable `SPA_DIST`. Optional |
| `KUSTOMIZE_VERSION` | Kustomize release the manifest jobs install. Default `5.6.0` | GitLab CI/CD variable `KUSTOMIZE_VERSION`. Optional |
| `KEYCLOAK_ISSUER` | Issuer URL of the existing realm | `deploy/configmap.yaml`. Replace `https://placeholder.invalid/realms/placeholder-realm`. Not a secret |
| `KEYCLOAK_CLIENT_ID` | Portal client id in that realm | `deploy/configmap.yaml`. Replace `placeholder-client-id`. Not a secret |
| `KEYCLOAK_HR_GROUP` | Keycloak group that is HR | `deploy/configmap.yaml`. Replace `placeholder-hr-group`. Not a secret |
| `SMTP_HOST` | Host of the existing SMTP server | `deploy/configmap.yaml`. Replace `placeholder.invalid`. Not a secret |
| `SMTP_PORT` | SMTP port | `deploy/configmap.yaml`. Replace `placeholder-smtp-port`. Not a secret |
| `SMTP_TLS` | TLS mode the worker accepts: `starttls`, `tls`, or `none` | `deploy/configmap.yaml`. Replace `placeholder-smtp-tls`. Not a secret |
| `SMTP_FROM` | Mail From address | `deploy/configmap.yaml`. Replace `placeholder@placeholder.invalid`. Not a secret |
| `HTTP_ADDR` | API listen address | `deploy/configmap.yaml`. Value `:8080` |
| `STATIC_DIR` | SPA files inside the image | `deploy/configmap.yaml`. Value `/app/static` |
| `LOG_LEVEL` | Log level | `deploy/configmap.yaml`. Value `info` |
| `placeholder-vault-host.invalid` | Vault API server. The URL in `deploy/secretstore.yaml` is `https://placeholder-vault-host.invalid`. Include the port in that URL when the server uses one | `deploy/secretstore.yaml` `server` |
| `placeholder-vault-mount` | KV v2 mount name | `deploy/secretstore.yaml` `path` |
| `placeholder-vault-k8s-auth-mount` | Vault Kubernetes auth mount | `deploy/secretstore.yaml` `auth.kubernetes.mountPath` |
| `placeholder-vault-role` | Vault role bound to ServiceAccount `bkl-vacation-vault` in the portal namespace | `deploy/secretstore.yaml` `auth.kubernetes.role` |
| `placeholder-cluster-name` | Name of the cluster as Argo CD has it registered | `deploy/argocd/application.yaml` `destination.name` |
| `placeholder-namespace` | Kubernetes namespace. Keep the Kustomize `namespace` and the Application `destination.namespace` the same | `deploy/kustomization.yaml` and `deploy/argocd/application.yaml` |
| `placeholder-ingress-class` | Ingress class | `deploy/ingress.yaml` |
| `placeholder-portal-host.invalid` | Portal DNS name | `deploy/ingress.yaml` host |
| `placeholder-gitlab-group` | GitLab group path | Not a cluster value. The GitLab project that runs `.gitlab-ci.yml` |
| `placeholder-gitlab-project` | GitLab project path | Same GitLab project |

`metadata.namespace: argocd` on the Application is the upstream Argo CD install namespace. Change it if this cluster's Argo CD lives somewhere else. `spec.project: default` is Argo CD's built-in AppProject. Change it if the portal has its own project.

The Application `repoURL` is `https://github.com/wgh-control/bkl-vacation.git`, revision `main`, path `deploy`. If Argo CD should watch the GitLab project instead, replace `repoURL` once that project exists, and set `GIT_PUSH_URL` to the same remote.

Vault policy for the portal role, once the mount name is real:

```hcl
path "placeholder-vault-mount/data/bkl-vacation/runtime" {
  capabilities = ["read"]
}
```

The Kubernetes auth role binds `bound_service_account_names` to `bkl-vacation-vault` and `bound_service_account_namespaces` to the portal namespace. A separate CI policy can read `placeholder-vault-mount/data/bkl-vacation/ci` when GitLab's Vault integration replaces `REGISTRY_USER`, `REGISTRY_PASSWORD`, and `GIT_PUSH_TOKEN`. That integration is not enabled in `.gitlab-ci.yml` yet, so those three still come from GitLab CI/CD variables.

If Vault's certificate is signed by a private CA, add that CA on the SecretStore. The bundle is not committed.

## Git and CI

Application source and these manifests live in `wgh-control/bkl-vacation`. The recommended GitLab setup is a pull mirror of that GitHub repository, with this `.gitlab-ci.yml`. The GitLab group has not been chosen.

Stages, in order:

1. Test. `go test ./...`, `go vet ./...`, and `gofmt` when `go.mod` is present. `npm ci`, `npm test`, and `npm run lint` when `frontend/package.json` is present. `kustomize build deploy` on every pipeline.
2. Build the image from the root `Dockerfile`.
3. Push to the registry, tagged with the commit SHA. The publish job refuses a `latest` tag.
4. `kustomize edit set image` updates `deploy/kustomization.yaml` and pushes that commit with `[skip ci]`. Argo CD syncs the new tag.

Publish and the tag commit run on the default branch only. They run once `go.mod`, `frontend/package.json`, and `frontend/package-lock.json` are all in the revision. Until those files exist, the pipeline still renders the manifests and leaves the image tag unchanged.

With the recommended pull mirror, `GIT_PUSH_URL` is `https://github.com/wgh-control/bkl-vacation.git`. Push the tag commit to that remote. A push to the GitLab pull mirror does not update the revision Argo CD watches.

Build and publish use Docker-in-Docker (`docker:27`). The runner must allow privileged mode for that service.

## Image contract

One image. The API and the worker are the same build.

| Command | Process |
| --- | --- |
| `/app/api` | HTTP API and the static SPA. This is the image entrypoint |
| `/app/api migrate` | Apply schema migrations to the external database, then exit 0. The PreSync Job runs this and must succeed before the API and the worker roll |
| `/app/worker` | Directory sync, notices, and SMTP. One replica in this tree |

The Dockerfile builds `./cmd/api` and `./cmd/worker` from the Go module at the repository root, and runs `npm ci` then `npm run build` in `frontend/`. The frontend build uses `package-lock.json`. Its output directory is `frontend/dist` unless `SPA_DIST` is set. Those files are copied to `/app/static`.

The API serves `GET /healthz` and `GET /readyz` on `HTTP_ADDR` (`:8080` in the ConfigMap). `/healthz` answers when the process is up and does not require the database, so liveness does not restart the pod during a database blip. `/readyz` checks Postgres. The same port serves `/api` and the SPA files from `STATIC_DIR` (`/app/static`).

The API Deployment starts at 2 replicas so a rolling update can keep a pod ready. That number is a starting value, not a capacity estimate. The worker starts at 1 replica and uses the `Recreate` strategy. Resource requests and limits are starting values.

When the registry requires a pull credential, add the cluster's pull secret to each Pod spec:

```yaml
imagePullSecrets:
  - name: <pull secret name>
```

The secret name is part of the registry decision and is not set here.

## What Argo CD syncs

`kustomize build deploy` is the desired state. External Secrets Operator is the Vault integration. Vault Agent Injector is not used: the processes read environment variables, and the image has no shell to source a Vault file.

| Resource | Sync |
| --- | --- |
| ServiceAccount `bkl-vacation-vault` | PreSync hook, wave -4 |
| SecretStore `bkl-vacation-vault` | PreSync hook, wave -3 |
| ExternalSecret `bkl-vacation` | PreSync hook, wave -2. Writes Secret `bkl-vacation` from Vault and leaves that Secret in place when the hook object is replaced |
| Migration Job `bkl-vacation-migrate` | PreSync hook, wave -1. Deleted before the next hook run |
| ServiceAccount `bkl-vacation` and ConfigMap `bkl-vacation` | wave 0 |
| Deployment `bkl-vacation-api`, Service `bkl-vacation`, Ingress `bkl-vacation` | wave 1 |
| Deployment `bkl-vacation-worker` | wave 2 |

Argo CD treats a hook as done when the object is Healthy. A ServiceAccount and a SecretStore are Healthy once they exist. An ExternalSecret is Healthy once Vault has filled the Secret only after the platform adds a health check. Merge the contents of `deploy/argocd/external-secret-health.lua` into the existing `argocd-cm` ConfigMap under the data key `resource.customizations.health.external-secrets.io_ExternalSecret`. Do not replace that ConfigMap with a new one. Without the health check, the migration Job can start before Secret `bkl-vacation` exists.

The ExternalSecret refreshes from Vault every hour. A running pod keeps the environment it started with, so a rotated value is picked up on the next pod start.

The Service is ClusterIP. The Ingress sends `/` to it, so the SPA and the API share one host. Add a TLS section on the Ingress when the certificate and the DNS name exist. Neither is set here.

The API, worker, and migration pods need egress to cluster DNS, the external Postgres, the existing Keycloak, and the existing SMTP server. They do not call Vault. External Secrets Operator needs a path to the Vault server.

Pods run as uid 65532 with a read-only root filesystem and a dropped capability set. They do not mount a Kubernetes API token. External Secrets uses the TokenRequest API for ServiceAccount `bkl-vacation-vault`, so that account does not mount a token either.

## Argo CD Application

`deploy/argocd/application.yaml` is applied once by whoever administers Argo CD, after the cluster name and the other placeholders exist:

```sh
kubectl apply -f deploy/argocd/application.yaml
```

The committed sync policy is automated, with `prune: false` and `selfHeal: false`. After the first production sync has been reviewed by hand, set `selfHeal: true`. Leave `prune` false through that review. Secret `bkl-vacation` is created by External Secrets with `creationPolicy: Orphan` and `deletionPolicy: Retain`, so replacing the ExternalSecret hook does not delete it.

The namespace is not created by this Application. Create the portal namespace before the first sync.

`deploy/examples/secret.yaml` only names the runtime keys. It is not applied.
