#!/usr/bin/env bash
# Publishing is invoked by the authorized release workflow, never by packaging.
set -euo pipefail
tag="${1:-}"
assets="${2:-}"
notes="${3:-}"
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ && -d "$assets" && -f "$notes" ]] || exit 2
repository="nanachi1212/mystocktracer"
if gh release view "$tag" --repo "$repository" >/dev/null 2>&1; then
  gh release upload "$tag" "$assets"/* --repo "$repository" --clobber
  gh release edit "$tag" --repo "$repository" --title "mystocktracer $tag" --notes-file "$notes"
else
  gh release create "$tag" "$assets"/* --repo "$repository" --verify-tag --title "mystocktracer $tag" --notes-file "$notes"
fi
# Existing unrelated release assets are deliberately retained.
gh release view "$tag" --repo "$repository" --json url