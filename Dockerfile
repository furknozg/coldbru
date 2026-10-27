# One image per service: docker build --build-arg SERVICE=proposal . (D-021, D-056)
FROM golang:1.27-alpine AS build
ARG SERVICE
RUN test -n "${SERVICE}" || (echo "build arg SERVICE is required, e.g. --build-arg SERVICE=proposal" >&2; exit 1)
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
