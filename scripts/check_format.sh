#!/bin/bash

# Script to check code formatting without applying changes
# Useful for CI environments

set -e

echo "Checking Go code formatting..."

# Get GOPATH for finding tools
GOPATH=$(go env GOPATH)
GOFUMPT_PATH="${GOPATH}/bin/gofumpt"
GOIMPORTS_PATH="${GOPATH}/bin/goimports"

# Check if gofumpt is installed
if [ -x "$GOFUMPT_PATH" ]; then
  echo "Using gofumpt for better formatting..."
  UNFORMATTED=$("$GOFUMPT_PATH" -l .)
else
  echo "gofumpt not found, using standard gofmt..."
  UNFORMATTED=$(gofmt -l .)
fi

# Check if there are unformatted files
if [ -n "$UNFORMATTED" ]; then
  echo "The following files are not formatted correctly:"
  echo "$UNFORMATTED"
  echo -e "\nPlease run 'make fmt' to format these files."
  exit 1
else
  echo "All Go files are properly formatted."
fi

# Check imports if goimports is installed
if [ -x "$GOIMPORTS_PATH" ]; then
  echo -e "\nChecking import formatting..."
  IMPORT_ISSUES=$("$GOIMPORTS_PATH" -l -local github.com/theblitlabs/parity-runner .)
  if [ -n "$IMPORT_ISSUES" ]; then
    echo "The following files have import formatting issues:"
    echo "$IMPORT_ISSUES"
    echo -e "\nPlease run 'make imports' to fix these issues."
    exit 1
  else
    echo "All imports are properly formatted."
  fi
fi

echo -e "\nFormat check successful!"
exit 0