#!/usr/bin/env bash

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
NAME=debian-12
LIMA_CONFIG_PATH="$SCRIPT_DIR/debian-12.yaml"
KEY_DIR=$(mktemp -d)

cleanup() {
  echo "Cleaning up..."
  limactl delete --force "$NAME" 2>/dev/null || true
  rm -rf "$KEY_DIR"
}
trap cleanup EXIT

ssh-keygen -t ed25519 -f "$KEY_DIR/id_ed25519" -N "" -q
SSH_PUBLIC_KEY=$(cat "$KEY_DIR/id_ed25519.pub")

echo "Creating Lima VM..."
limactl create --tty=false --name="$NAME" \
  --set ".param.SSHPublicKey=\"$SSH_PUBLIC_KEY\"" \
  "$LIMA_CONFIG_PATH"
limactl start "$NAME"

TEST_SSH_HOST=$(limactl shell "$NAME" \
  ip -j -f inet addr show lima1 | jq -r '.[0].addr_info[0].local')

echo "VM ready at $TEST_SSH_HOST"

export TF_ACC=1
export TEST_SSH_HOST
export TEST_SSH_PRIVATE_KEY
TEST_SSH_PRIVATE_KEY=$(cat "$KEY_DIR/id_ed25519")
export TEST_SSH_PUBLIC_KEY
TEST_SSH_PUBLIC_KEY=$(awk '{print $1, $2}' "$KEY_DIR/id_ed25519.pub")

echo "Running tests..."
(cd "$PROJECT_DIR" && go test -v -count=1 ./...)
