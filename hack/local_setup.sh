#!/usr/bin/env bash

# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

# Brings up the full local Gardener dev environment and deploys this extension.
#
# Prerequisites:
#   - ../gardener checked out (sibling directory)
#   - Docker running with sufficient resources (8 CPUs, 8Gi RAM, 120Gi disk)
#   - gcloud authenticated (gcloud auth login) if pulling from europe-docker.pkg.dev
#
# What it does:
#   1. make kind-up         — creates the kind cluster + local registry + DNS/loopback setup
#   2. make gardener-up     — deploys operator, garden, gardenlet/seed (all-in-one)
#   3. make extension-up    — builds and deploys the extension via skaffold
#
# After this script, create a shoot with:
#   kubectl --kubeconfig ../gardener/dev-setup/kubeconfigs/virtual-garden/kubeconfig \
#     apply -f ../gardener/example/provider-local/shoot.yaml
#
# IMPORTANT: Never run 'kubectl apply -f hack/local-setup/operator-extension-resource.yaml'
# directly! That file has bare 'local-skaffold/...' refs that only skaffold can resolve.
# Always use 'make extension-up' instead (which is what this script does).
#
# Kubeconfigs (relative to ../gardener):
#   Runtime cluster:  dev-setup/kubeconfigs/runtime/kubeconfig
#   Virtual garden:   dev-setup/kubeconfigs/virtual-garden/kubeconfig

set -o nounset
set -o pipefail
set -o errexit
set -x

repo_root="$(readlink -f "$(dirname "${0}")/..")"
gardener_dir="${repo_root}/../gardener"

cd "${gardener_dir}"

echo ">>>>>>>>>>>>>>>>>>>> kind-up"
make kind-up
echo "<<<<<<<<<<<<<<<<<<<< kind-up done"

export KUBECONFIG="${gardener_dir}/dev-setup/kubeconfigs/runtime/kubeconfig"

echo ">>>>>>>>>>>>>>>>>>>> gardener-up"
make gardener-up
echo "<<<<<<<<<<<<<<<<<<<< gardener-up done"

cd "${repo_root}"

# extension-up pushes to the in-cluster registry, so stay on the runtime cluster.

echo ">>>>>>>>>>>>>>>>>>>> extension-up"
make extension-up
echo "<<<<<<<<<<<<<<<<<<<< extension-up done"

echo ""
echo "Local dev environment is ready."
echo "Create a shoot with:"
echo "  kubectl --kubeconfig ${gardener_dir}/dev-setup/kubeconfigs/virtual-garden/kubeconfig apply -f ${gardener_dir}/example/provider-local/shoot.yaml"
