#! /bin/zsh
#

set -e

MANUALDIR="manual"
MDDIR="${MANUALDIR}/md"
MANPAGEDIR="${MANUALDIR}/man"
HTMLDIR="${MANUALDIR}/html"

go build

mkdir -p "$MDDIR" "$MANPAGEDIR" "$HTMLDIR"

./doc "$MDDIR" "$MANPAGEDIR"

for f in "$MDDIR"/*.md; do pandoc --from=markdown --to=html "$f" >"$HTMLDIR/${${f:t}%md}html"; done

sed -i 's/\.md"/.html"/g' "$HTMLDIR"/*.html

(cd "$HTMLDIR" && ln -sf samtvcli.html index.html)
