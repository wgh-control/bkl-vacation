# One image, two processes: /app/api and /app/worker.
# The migration Job runs `/app/api migrate`.
# SPA files are copied to /app/static. See deploy/README.md.

ARG GO_VERSION=1.25
ARG NODE_VERSION=22
ARG SPA_DIST=dist

FROM golang:${GO_VERSION}-bookworm AS go-build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
    && chmod 755 /out/api /out/worker

FROM node:${NODE_VERSION}-bookworm AS spa

ARG SPA_DIST=dist
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build && chmod -R a+rX "${SPA_DIST}"

FROM gcr.io/distroless/static-debian12:nonroot

ARG SPA_DIST=dist
WORKDIR /app
COPY --from=go-build /out/api /app/api
COPY --from=go-build /out/worker /app/worker
COPY --from=spa /src/frontend/${SPA_DIST} /app/static
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/api"]
