#!/bin/bash
# Usage: ./backfill.sh <num_commits> <repo_path>
# Example: ./backfill.sh 20 /mnt/ephemeral/workdir/scio/scio

set -e

N=${1:-10}
REPO_PATH=${2:-.}
BAZEL_PATH=/usr/local/bin/bazel
TOPOSCOPE=~/toposcope
REPO_NAME="askscio/scio"
API_KEY="1c1a0904bf3a7006cba1beb3eecaca5eb18f73a28ba72ae1cd7d4bbf9dfecf06"
SERVICE_URL="https://toposcope-service-bqbgmvspcq-uc.a.run.app"

cd "$REPO_PATH"

# Save original ref so we can restore after toposcope detaches HEAD
ORIG_BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || git rev-parse HEAD)

restore_head() {
  echo "Restoring HEAD to $ORIG_BRANCH"
  git checkout "$ORIG_BRANCH" --quiet 2>/dev/null || true
}
trap restore_head EXIT

# Get GCP identity token for Cloud Run auth
get_token() {
  curl -s -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=$SERVICE_URL"
}

# Collect commit SHAs from the branch tip (oldest first)
COMMITS=$(git log --reverse --format=%H -n "$N" "$ORIG_BRANCH")
COMMIT_ARRAY=($COMMITS)

echo "Backfilling ${#COMMIT_ARRAY[@]} commits from $ORIG_BRANCH"

for ((i=1; i<${#COMMIT_ARRAY[@]}; i++)); do
  BASE_SHA=${COMMIT_ARRAY[$((i-1))]}
  HEAD_SHA=${COMMIT_ARRAY[$i]}

  echo ""
  echo "=== [$i/$((${#COMMIT_ARRAY[@]}-1))] ${BASE_SHA:0:7}..${HEAD_SHA:0:7} ==="

  # Restore to HEAD_SHA before scoring so toposcope can checkout base/head
  git checkout "$HEAD_SHA" --quiet

  # Score (uses full SHAs, not HEAD/HEAD~1)
  $TOPOSCOPE score \
    --base "$BASE_SHA" \
    --head "$HEAD_SHA" \
    --bazel-path "$BAZEL_PATH" \
    --repo-path "$REPO_PATH" \
    --output json > /tmp/score.json 2>/tmp/score.log || {
      echo "  WARN: score failed, skipping"
      cat /tmp/score.log
      continue
    }

  # Find cached snapshots
  CACHE_DIR=$(find ~/.cache/toposcope -type d -name snapshots | head -1)

  if [ ! -f "$CACHE_DIR/$HEAD_SHA.json" ] || [ ! -f "$CACHE_DIR/$BASE_SHA.json" ]; then
    echo "  WARN: snapshots not cached, skipping"
    continue
  fi

  # Get the commit timestamp (RFC3339)
  COMMITTED_AT=$(git show -s --format=%cI "$HEAD_SHA")

  # Build payload
  jq -n \
    --arg repo "$REPO_NAME" \
    --arg sha "$HEAD_SHA" \
    --arg committed_at "$COMMITTED_AT" \
    --slurpfile head "$CACHE_DIR/$HEAD_SHA.json" \
    --slurpfile base "$CACHE_DIR/$BASE_SHA.json" \
    --slurpfile score /tmp/score.json \
    '{
      repo_full_name: $repo,
      default_branch: "main",
      commit_sha: $sha,
      branch: "main",
      committed_at: $committed_at,
      snapshot: $head[0],
      base_snapshot: $base[0],
      score: $score[0]
    }' > /tmp/ingest.json

  # Gzip the payload (70MB+ JSON compresses to ~3MB)
  gzip -kf /tmp/ingest.json
  SIZE=$(wc -c < /tmp/ingest.json.gz)
  echo "  Posting ${SIZE} bytes (gzipped)..."

  # Fresh token for each request (tokens expire after 1h)
  TOKEN=$(get_token)

  HTTP_CODE=$(curl -s -o /tmp/response.json -w "%{http_code}" \
    -X POST "$SERVICE_URL/api/v1/ingest" \
    -H "Content-Type: application/json" \
    -H "Content-Encoding: gzip" \
    -H "Authorization: Bearer $TOKEN" \
    -H "X-API-Key: $API_KEY" \
    --data-binary @/tmp/ingest.json.gz)

  echo "  Response ($HTTP_CODE): $(cat /tmp/response.json)"

  if [ "$HTTP_CODE" -ge 400 ]; then
    echo "  ERROR: ingest failed"
  fi
done

echo ""
echo "Done. Backfilled $((${#COMMIT_ARRAY[@]} - 1)) commit pairs."
