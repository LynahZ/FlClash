#!/usr/bin/env bash
set -euo pipefail

release=$1
tag=$2
core_upstream=$3
bundles=$4
core=core/Clash.Meta
base=$(git rev-parse HEAD)

if git merge-base --is-ancestor "$release" "$base"; then
  echo "$tag is already in the fork."
  exit 0
fi

old_core=$(git rev-parse "$base:$core")
released_core=$(git rev-parse "$release:$core")
git submodule update --init -- "$core"

if ! git -c submodule.recurse=false merge --no-ff --no-commit "$release"; then
  git rev-parse --verify --quiet MERGE_HEAD >/dev/null
  if [[ $(git diff --name-only --diff-filter=U) != "$core" ]]; then
    echo "::error::FlClash $tag has merge conflicts outside the core gitlink."
    exit 1
  fi
fi

if [[ $(git config -f .gitmodules --get submodule.core/Clash.Meta.url) != ../FlClash.git ||
      $(git config -f .gitmodules --get submodule.core/Clash.Meta.branch) != macc-core ]]; then
  echo "::error::FlClash $tag changed how the MACC core is tracked."
  exit 1
fi

git -C "$core" checkout --detach "$old_core"
if ! git -C "$core" merge-base --is-ancestor "$released_core" "$old_core" 2>/dev/null; then
  git -C "$core" fetch --no-tags "$core_upstream" "$released_core"
  git -C "$core" merge --no-ff -m "chore: merge FlClash $tag core" "$released_core"
fi
new_core=$(git -C "$core" rev-parse HEAD)
git -C "$core" merge-base --is-ancestor "$old_core" "$new_core"
git -C "$core" merge-base --is-ancestor "$released_core" "$new_core"
git add -- "$core"
whitespace_paths=(.)
while IFS= read -r -d '' path; do
  case "${path##*/}" in
    LICENSE|LICENSE.*|LICENSE-*|COPYING|COPYING.*|COPYING-*|OFL|OFL.txt)
      if git diff --cached --quiet "$release" -- ":(top,literal)$path"; then
        whitespace_paths+=(":(exclude,top,literal)$path")
      fi
      ;;
  esac
done < <(git diff --cached --name-only -z)
git diff --cached --check -- "${whitespace_paths[@]}"
git commit -m "chore: sync upstream FlClash $tag"

mkdir -p "$bundles"
git bundle create "$bundles/application.bundle" HEAD "^$base"
if [[ $old_core != "$new_core" ]]; then
  git -C "$core" bundle create "$bundles/core.bundle" HEAD "^$old_core"
fi
