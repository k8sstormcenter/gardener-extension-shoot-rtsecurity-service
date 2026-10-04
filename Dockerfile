# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

############# builder
FROM golang:1.26.5 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-shoot-rtsecurity-service

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

############# gardener-extension-admission-shoot-rtsecurity-service
FROM base AS gardener-extension-admission-shoot-rtsecurity-service

WORKDIR /
COPY --from=builder /go/bin/gardener-extension-admission-shoot-rtsecurity-service /gardener-extension-admission-shoot-rtsecurity-service
ENTRYPOINT ["/gardener-extension-admission-shoot-rtsecurity-service"]

############# falco-ops-builder
FROM alpine:3.24.1 AS falco-ops-builder

RUN mkdir -p /volume/bin /volume/lib /volume/tmp \
    && cp /bin/busybox /volume/bin/                   && echo "package busybox" \
    && cp -d /lib/ld-musl-* /volume/lib/              && echo "package musl" \
    && for cmd in sh awk date echo grep head sed sleep wget; do \
         ln -s busybox /volume/bin/$cmd; \
       done

############# falco-ops
FROM scratch AS falco-ops
WORKDIR /
COPY --from=falco-ops-builder /volume .
