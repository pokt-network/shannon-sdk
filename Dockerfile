# Multi-stage build for both crypto backend variants
FROM golang:1.21-alpine AS builder

# Install CGO dependencies for Ethereum backend
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY . .

# Build fast version (Ethereum backend)
RUN go build -tags="ethereum_secp256k1" -o shannon-sdk-fast

# Build portable version (Decred backend)
RUN CGO_ENABLED=0 go build -o shannon-sdk-portable

# Final image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Choose which binary to include:
# For maximum performance (requires CGO dependencies):
COPY --from=builder /app/shannon-sdk-fast ./shannon-sdk
# For maximum portability (uncomment this instead):
# COPY --from=builder /app/shannon-sdk-portable ./shannon-sdk

CMD ["./shannon-sdk"]