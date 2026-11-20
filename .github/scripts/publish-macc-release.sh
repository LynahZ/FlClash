#!/usr/bin/env bash
set -euo pipefail

artifacts=$1
dist=$2
tag="$UPSTREAM_TAG-macc"

[[ "$GH_REPO" == LynahZ/FlClash ]]
[[ "$UPSTREAM_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
[[ "$REVISION" =~ ^[0-9a-f]{40}$ && "$SOURCE_RUN" =~ ^[0-9]+$ ]]

mkdir -p "$dist"
jq -es --arg revision "$REVISION" --arg version "${UPSTREAM_TAG#v}" '
  [.[].artifacts[]] | sort_by(.os, .arch, .form) |
  if map([.os, .arch, .form, (.name | split(".") | last)]) != [
    ["android", "arm64", null, "apk"],
    ["macos", "amd64", null, "dmg"],
    ["macos", "arm64", null, "dmg"],
    ["windows", "amd64", "portable", "zip"],
    ["windows", "amd64", "setup", "exe"],
    ["windows", "arm64", "portable", "zip"],
    ["windows", "arm64", "setup", "exe"]
  ] then error("Incomplete or unexpected package matrix")
  elif (map(.version) | unique | length) != 1 or (map(.line) | unique | length) != 1
  then error("Packages disagree on build version or release line")
  elif (map(.name) | unique | length) != length
  then error("More than one package has the same filename")
  elif any(.[]; .commit != $revision or
    (.version != $version and (.version | startswith($version + "-") | not)) or
    (.name | test("^FlClash-[A-Za-z0-9.-]+$") | not) or
    (.sha256 | test("^[a-f0-9]{64}$") | not))
  then error("Unexpected package metadata")
  else . end
' "$artifacts"/FlClash-MACC-*/manifest.json > "$dist/packages.json"

while IFS=$'\t' read -r os arch name digest; do
  cp -- "$artifacts/FlClash-MACC-$os-$arch/$name" "$dist/$name"
  printf '%s  %s\n' "$digest" "$name"
done < <(jq -r '.[] | [.os, .arch, .name, .sha256] | @tsv' "$dist/packages.json") > "$dist/SHA256SUMS"

(
  cd "$dist"
  sha256sum --check --strict SHA256SUMS
)

existing=$(gh api --paginate "repos/$GH_REPO/releases" \
  --jq ".[] | select(.tag_name == \"$tag\")")
if [[ -n "$existing" ]] && jq -e '.draft == false' <<< "$existing" > /dev/null; then
  echo "$tag is already published."
  exit 0
fi

remote_refs=$(git ls-remote "https://github.com/$GH_REPO.git" "refs/tags/$tag" "refs/tags/$tag^{}")
if [[ -n "$remote_refs" ]]; then
  remote_revision=$(tail -n 1 <<< "$remote_refs" | cut -f1)
  if [[ "$remote_revision" != "$REVISION" ]]; then
    echo "::error::$tag already points to $remote_revision instead of $REVISION."
    exit 1
  fi
else
  gh api --method POST "repos/$GH_REPO/git/refs" \
    -f "ref=refs/tags/$tag" -f "sha=$REVISION" > /dev/null
fi

if [[ -z "$existing" ]]; then
  gh release create "$tag" --repo "$GH_REPO" --draft --verify-tag \
    --title "FlClash $UPSTREAM_TAG with MACC" --notes-file - <<EOF
Based on [upstream FlClash $UPSTREAM_TAG](https://github.com/chen08209/FlClash/releases/tag/$UPSTREAM_TAG).
Includes MACC support. [GitHub Actions build](https://github.com/$GH_REPO/actions/runs/$SOURCE_RUN).

- macOS Apple Silicon and Intel (DMG)
- Windows x64 and ARM64 (installer and portable ZIP)
- Android ARM64 (APK)

The Android APK is signed with the fork's release key and uses
\`com.follow.clash.macc\`. Install it once alongside the official app or the
earlier debug build, then update it in place with newer MACC releases.
EOF
fi

assets=("$dist/SHA256SUMS")
while IFS= read -r name; do
  assets+=("$dist/$name")
done < <(jq -r '.[].name' "$dist/packages.json")
gh release upload "$tag" --repo "$GH_REPO" --clobber "${assets[@]}"
gh release edit "$tag" --repo "$GH_REPO" --draft=false --latest
