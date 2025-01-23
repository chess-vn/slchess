# Stage 1: Build
FROM golang:1.23.5-alpine AS builder
WORKDIR /app

# Cache module downloads
ENV GOCACHE=/go-cache
ENV GOMODCACHE=/gomod-cache
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/gomod-cache \
  go mod download

# Copy source code and build
COPY ./ ./
RUN --mount=type=cache,target=/gomod-cache --mount=type=cache,target=/go-cache \
  go build -v -o server ./cmd/server/

# Stage 2: Runtime
FROM scratch
COPY --from=builder /app/server /bin/server
EXPOSE 7202
ENTRYPOINT ["/bin/server"]
