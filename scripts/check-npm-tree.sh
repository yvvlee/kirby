#!/bin/sh
set -eu

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-npm-tree.XXXXXX")

cleanup() {
  rm -rf "$work_dir"
}
trap cleanup EXIT INT TERM

npm_tree="$work_dir/tree.json"
npm_tree_error="$work_dir/tree.error"
if (cd "$repo_dir/web" && npm ls --all --json > "$npm_tree" 2> "$npm_tree_error"); then
  exit 0
fi

jq -e --arg web_dir "$repo_dir/web" '
  .problems == ["invalid: typescript@6.0.3 " + $web_dir + "/node_modules/typescript"]
  and .dependencies.typescript.version == "6.0.3"
  and .dependencies["@formily/antd-v5"].version == "1.2.4"
  and .dependencies["@formily/antd-v5"].dependencies["@formily/grid"].version == "2.3.7"
  and .dependencies["@formily/antd-v5"].dependencies["@formily/grid"]
    .dependencies.typescript.invalid
    == "\"4.x || 5.x\" from node_modules/@formily/antd-v5/node_modules/@formily/grid"
' "$npm_tree" >/dev/null || {
  cat "$npm_tree_error" >&2
  exit 1
}

echo "Accepting the reviewed @formily/grid TypeScript 6 peer-range exception."
(cd "$repo_dir/web" && npm test -- --run src/spike/formily-compatibility.test.tsx)
