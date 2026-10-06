# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

############# builder
FROM golang:1.26.5 AS builder

WORKDIR /go/src/github.com/k8sstormcenter/gardener-extension-shoot-rtsecurity-service

# Copy go mod and sum files
COPY go.mod go.sum ./
# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

COPY . .

ARG EFFECTIVE_VERSION
RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

############# base
FROM gcr.io/distroless/static-debian12:nonroot AS base

############# gardener-extension-shoot-rtsecurity-service
FROM base AS gardener-extension-shoot-rtsecurity-service

WORKDIR /
COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-shoot-rtsecurity-service /gardener-extension-shoot-rtsecurity-service
ENTRYPOINT ["/gardener-extension-shoot-rtsecurity-service"]
