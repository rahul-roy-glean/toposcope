# ---------------------------------------------------------
# Multi-stage Dockerfile for the toposcope binary.
#
# Build:
#   docker build -t toposcope .
#
# Run (example – starts the UI server on port 7700):
#   docker run -p 7700:7700 toposcope ui --port 7700
# ---------------------------------------------------------

# Stage 1 — build the Go binaries
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Cache module downloads before copying the full source tree.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build both binaries with static linking and stripped debug info.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/toposcope  ./cmd/toposcope \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/toposcoped ./cmd/toposcoped

# Stage 2 — minimal runtime image
FROM alpine:3.20

# ca-certificates: TLS connections to external services.
# git:             required by toposcope for repository analysis.
RUN apk add --no-cache ca-certificates git

COPY --from=builder /out/toposcope  /usr/local/bin/toposcope
COPY --from=builder /out/toposcoped /usr/local/bin/toposcoped

EXPOSE 7700

ENTRYPOINT ["toposcope"]
