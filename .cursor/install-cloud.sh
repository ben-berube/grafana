#!/usr/bin/env bash
# Install the Grafana frontend toolchain for a Cloud Agent build.
set -euo pipefail

NODE_VERSION="$(tr -d '[:space:]' < .nvmrc)"
if ! command -v node >/dev/null 2>&1 || [[ "$(node -v)" != "v${NODE_VERSION}" ]]; then
  curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" -o /tmp/node.tar.xz
  if [[ "$(id -u)" -eq 0 ]]; then
    tar -xJf /tmp/node.tar.xz -C /usr/local --strip-components=1
  else
    sudo tar -xJf /tmp/node.tar.xz -C /usr/local --strip-components=1
  fi
fi

corepack enable
corepack install
yarn install --immutable
