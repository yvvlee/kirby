#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
workflow_dir="$repo_dir/.github/workflows"

if grep -R -n -E 'uses:[[:space:]]+[^@[:space:]]+@(main|master|v[0-9]+([.][0-9]+){0,2})[[:space:]]*(#.*)?$' "$workflow_dir"; then
  echo "workflow action is not pinned to a full commit" >&2
  exit 1
fi
if grep -R -n -E 'git[.]changbaops[.]com|registry[.]changba|(^|[^[:alnum:]_])(old|dashboard)/' \
  "$workflow_dir" "$repo_dir/scripts/ci"; then
  echo "workflow references a private or excluded source" >&2
  exit 1
fi
action_count=$(grep -R -h -E 'uses:[[:space:]]+[^@[:space:]]+@[0-9a-f]{40}([[:space:]]|$)' "$workflow_dir" | wc -l | tr -d ' ')
test "$action_count" -ge 1
echo "Workflow action pins and public-source boundaries passed."
