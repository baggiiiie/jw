default:
    @just --list

build:
    go build -o bin/jw .

test:
    go test ./...

# Release with explicit version or auto-bump patch: just release [0.2.0]
release version="":
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{version}}" ]; then
        tag="v{{version}}"
    else
        latest=$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
        if [ -z "$latest" ]; then
            tag="v0.1.0"
        else
            IFS='.' read -r major minor patch <<< "${latest#v}"
            tag="v${major}.${minor}.$((patch + 1))"
        fi
    fi
    echo "Releasing ${tag}"
    git tag "$tag"
    git push origin "$tag"
