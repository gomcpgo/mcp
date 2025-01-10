#!/bin/bash

set -e

# Command line parsing
cmd=$1
shift

case "$cmd" in
    "test")
        go test -v ./...
        ;;
        
    "lint")
        go vet ./...
        if command -v golangci-lint &> /dev/null; then
            golangci-lint run
        else
            echo "golangci-lint not found. Please install it first."
            exit 1
        fi
        ;;
        
    "coverage")
        go test -coverprofile=coverage.out ./...
        go tool cover -html=coverage.out
        ;;
        
    "release")
        if [ -z "$1" ]; then
            echo "Please provide a version number (e.g., ./dev.sh release 0.2.0)"
            exit 1
        fi
        
        NEW_VERSION=$1
        
        # Validate version format (basic check)
        if ! [[ $NEW_VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "Invalid version format. Please use semantic versioning (e.g., 0.2.0)"
            exit 1
        fi
        
        # Update version.go
        sed -i.bak "s/Version = \"v[0-9]*\.[0-9]*\.[0-9]*\"/Version = \"v${NEW_VERSION}\"/" pkg/version/version.go
        rm -f pkg/version/version.go.bak
        
        # Commit and tag
        git add pkg/version/version.go
        git commit -m "Release v${NEW_VERSION}"
        git tag -a "v${NEW_VERSION}" -m "Release v${NEW_VERSION}"
        
        echo "Version ${NEW_VERSION} released. Don't forget to push with: git push && git push --tags"
        ;;
        
    *)
        echo "Usage: $0 [command]"
        echo "Commands:"
        echo "  test         Run tests"
        echo "  lint         Run linters"
        echo "  coverage     Generate test coverage report"
        echo "  release      Create a new release (requires version argument)"
        exit 1
        ;;
esac