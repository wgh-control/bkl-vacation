# Deploy

GitLab CI builds one image and pushes it tagged with the commit SHA. Argo CD syncs this directory onto Kubernetes. The pipeline does not apply manifests.

PostgreSQL, Keycloak, and SMTP stay outside the cluster. This tree has no database, identity provider, or mail server. Argo CD itself and the container registry are platform prerequisites. This Application does not install them.

## Placeholders

These strings are tokens. They are not a chosen cluster, GitLab group, or registry. Replace each one before the first production sync.

| Token | Replace with | Where |
| --- | --- | --- |
| `placeholder-gitlab-group` | GitLab group path | `.gitlab-ci.yml` header |
| `placeholder-gitlab-project` | GitLab project path | `.gitlab-ci.yml` header |
| `placeholder.invalid` | Container registry host | `deploy/kustomization.yaml` `newName`, CI variable `REGISTRY_HOST`, and the hosts inside the ConfigMap |
| `placeholder-image-name` | Image repository path under that host | `deploy/kustomization.yaml` `newName`, CI variable `IMAGE_NAME` |
| `placeholder-image-tag` | Full git commit SHA | `deploy/kustomization.yaml` `newTag`. CI writes this. Do not use `latest` |
| `placeholder-cluster-name` | Name of the cluster as Argo CD has it registered | `deploy/argocd/application.yaml` `destination.name` |
| `placeholder-namespace` | Kubernetes namespace for the portal | `deploy/kustomization.yaml` `namespace` and `destination.namespace`. Keep the two the same |
| `placeholder-ingress-class` | Ingress class | `deploy/ingress.yaml` |
| `placeholder-portal-host.invalid` | Portal DNS name | `deploy/ingress.yaml` host |
| `https://placeholder.invalid/realms/placeholder-realm` | Keycloak issuer URL of the existing realm | ConfigMap `KEYCLOAK_ISSUER` |
| `placeholder-client-id` | Keycloak client id | ConfigMap `KEYCLOAK_CLIENT_ID` |
| `placeholder-hr-group` | Keycloak group that is HR | ConfigMap `KEYCLOAK_HR_GROUP` |
| `placeholder-smtp-port` | SMTP port | ConfigMap `SMTP_PORT` |
| `placeholder-smtp-tls` | SMTP TLS mode the worker accepts (`starttls`, `tls`, or `none`) | ConfigMap `SMTP_TLS` |
| `placeholder@placeholder.invalid` | Mail From address | ConfigMap `SMTP_FROM` |

`metadata.namespace: argocd` on the Application is the upstream Argo CD install namespace. Change it if this cluster's Argo CD lives somewhere else. `spec.project: default` is Argo CD's built-in AppProject. Change it if the portal has its own project.

The Application `repoURL` is the known GitHub repository, `https://github.com/wgh-control/bkl-vacation.git`, revision `main`, path `deploy`. If Argo CD should watch the GitLab project instead, replace `repoURL` once that project exists, and point `GIT_PUSH_URL` at the same remote.

## Git and CI

Application source and these manifests live in `wgh-control/bkl-vacation`. The recommended GitLab setup is a pull mirror of that GitHub repository, with this `.gitlab-ci.yml`. The GitLab group has not been chosen.

Stages, in order:

1. Test. `go test ./...`, `go vet ./...`, and `gofmt` when `go.mod` is present. `npm ci`, `npm test`, and `npm run lint` when `frontend/package.json` is present. `kustomize build deploy` on every pipeline.
2. Build the image from the root `Dockerfile`.
3. Push to the registry, tagged with the commit SHA. The publish job refuses a `latest` tag.
4. `kustomize edit set image` updates `deploy/kustomization.yaml` and pushes that commit with `[skip ci]`. Argo CD syncs the new tag.

Publish and the tag commit run on the default branch only. They run once `go.mod`, `frontend/package.json`, and `frontend/package-lock.json` are all in the revision. Until those files exist, the pipeline still renders the manifests and leaves the image tag unchanged.

CI/CD variables to set on the GitLab project. Mask `REGISTRY_PASSWORD` and `GIT_PUSH_TOKEN`.

| Variable | Value |
| --- | --- |
| `REGISTRY_HOST` | Registry host. Replaces `placeholder.invalid` |
| `REGISTRY_USER` | Registry user |
| `REGISTRY_PASSWORD` | Registry password |
| `IMAGE_NAME` | Repository path under the host. Replaces `placeholder-image-name` |
| `GIT_PUSH_URL` | HTTPS remote of the git revision Argo CD watches, with no credentials in the URL |
| `GIT_PUSH_TOKEN` | Token that can push to `GIT_PUSH_URL` |
| `GIT_PUSH_USER` | Username for that token. Default `oauth2`. For GitHub use `x-access-token` |

With the recommended pull mirror, `GIT_PUSH_URL` is `https://github.com/wgh-control/bkl-vacation.git`. A push back to the GitLab mirror is the wrong remote: a pull mirror will not carry that commit to the revision Argo CD watches.

`GO_VERSION` defaults to `1.25`, `NODE_VERSION` to `22`, and `SPA_DIST` to `dist`. Match `GO_VERSION` to the `go` line in `go.mod`.

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

Environment variables:

| Name | Source | Purpose |
| --- | --- | --- |
| `HTTP_ADDR` | ConfigMap | API listen address |
| `STATIC_DIR` | ConfigMap | SPA files |
| `LOG_LEVEL` | ConfigMap | Log level. Default `info` |
| `KEYCLOAK_ISSUER` | ConfigMap | Existing realm's issuer URL |
| `KEYCLOAK_CLIENT_ID` | ConfigMap | Portal client id |
| `KEYCLOAK_HR_GROUP` | ConfigMap | HR group in that realm |
| `SMTP_HOST` | ConfigMap | Existing SMTP server |
| `SMTP_PORT` | ConfigMap | SMTP port |
| `SMTP_TLS` | ConfigMap | `starttls`, `tls`, or `none` |
| `SMTP_FROM` | ConfigMap | From address |
| `DATABASE_URL` | Secret | External Postgres DSN, with TLS |
| `KEYCLOAK_CLIENT_SECRET` | Secret | Portal client secret |
| `SMTP_USERNAME` | Secret | SMTP username |
| `SMTP_PASSWORD` | Secret | SMTP password |

The API Deployment starts at 2 replicas so a rolling update can keep a pod ready. That number is a starting value, not a capacity estimate. The worker starts at 1 replica and uses the `Recreate` strategy. Resource requests and limits are starting values.

When the registry requires a pull credential, add the cluster's pull secret to each Pod spec:

```yaml
imagePullSecrets:
  - name: <pull secret name>
```

The secret name is part of the registry decision and is not set here.

## What Argo CD syncs

`kustomize build deploy` is the desired state. It includes:

| Resource | Sync |
| --- | --- |
| ServiceAccount `bkl-vacation` and ConfigMap `bkl-vacation` | wave 0 |
| Migration Job `bkl-vacation-migrate` | PreSync hook, wave -1. Deleted before the next hook run |
| Deployment `bkl-vacation-api`, Service `bkl-vacation`, Ingress `bkl-vacation` | wave 1 |
| Deployment `bkl-vacation-worker` | wave 2 |

The Service is ClusterIP. The Ingress sends `/` to it, so the SPA and the API share one host. Add a TLS section on the Ingress when the certificate and the DNS name exist. Neither is set here.

The API, worker, and migration pods need egress to cluster DNS, the external Postgres, the existing Keycloak, and the existing SMTP server.

Pods run as uid 65532 with a read-only root filesystem and a dropped capability set. They do not mount a Kubernetes API token.

## Secret

Create Secret `bkl-vacation` in the portal namespace before the first sync. `deploy/examples/secret.yaml` lists the keys. It is not in the Kustomize build. How the Secret is delivered (Sealed Secrets, External Secrets, or another mechanism already on the cluster) waits on the cluster decision.

## Argo CD Application

`deploy/argocd/application.yaml` is applied once by whoever administers Argo CD, after the cluster name and the other placeholders exist:

```sh
kubectl apply -f deploy/argocd/application.yaml
```

Installing Argo CD, when it is not already on the cluster, is a platform prerequisite.

The committed sync policy is automated, with `prune: false` and `selfHeal: false`. After the first production sync has been reviewed by hand, set `selfHeal: true`. Leave `prune` false through that review. Prune only removes objects this Application used to manage. The out-of-band Secret is not one of those objects.

The namespace is not created by this Application. Create `placeholder-namespace` (the real name, once chosen) before the first sync.
