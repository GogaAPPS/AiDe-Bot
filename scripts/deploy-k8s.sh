#!/usr/bin/env bash
set -Eeuo pipefail

: "${IMAGE:?Usage: IMAGE=ghcr.io/gogaapps/aide-bot:tag scripts/deploy-k8s.sh}"

KUBECTL="${KUBECTL:-kubectl}"
NAMESPACE="${K8S_NAMESPACE:-apps}"
DEPLOYMENT="${K8S_DEPLOYMENT:-aide-bot}"
CONTAINER="${K8S_CONTAINER:-aide-bot}"

if ! "$KUBECTL" -n "$NAMESPACE" get deployment "$DEPLOYMENT" >/dev/null 2>&1; then
  echo "Deployment not found: $NAMESPACE/$DEPLOYMENT" >&2
  exit 1
fi

FOUND_CONTAINER="$("$KUBECTL" -n "$NAMESPACE" get deployment "$DEPLOYMENT" -o jsonpath='{.spec.template.spec.containers[0].name}' 2>/dev/null || true)"
if [ -n "$FOUND_CONTAINER" ]; then
  CONTAINER="$FOUND_CONTAINER"
fi

"$KUBECTL" -n "$NAMESPACE" set image \
  "deployment/$DEPLOYMENT" \
  "$CONTAINER=$IMAGE"

if ! "$KUBECTL" -n "$NAMESPACE" rollout status "deployment/$DEPLOYMENT" --timeout=180s; then
  echo "Rollout failed. Current Kubernetes state:" >&2
  "$KUBECTL" -n "$NAMESPACE" get deployment,pod >&2 || true
  "$KUBECTL" -n "$NAMESPACE" describe "deployment/$DEPLOYMENT" >&2 || true
  "$KUBECTL" -n "$NAMESPACE" get events --sort-by=.lastTimestamp | tail -n 30 >&2 || true
  exit 1
fi
