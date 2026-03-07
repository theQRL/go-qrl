# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

# Build Gqrl in a stock Go builder container
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev linux-headers git

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-qrl/
COPY go.sum /go-qrl/
RUN cd /go-qrl && go mod download

ADD . /go-qrl
RUN cd /go-qrl && go run build/ci.go install -static ./cmd/gqrl

# Pull Gqrl into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /go-qrl/build/bin/gqrl /usr/local/bin/

EXPOSE 8545 8546 30303 30303/udp
ENTRYPOINT ["gqrl"]

# Add some metadata labels to help programmatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"
