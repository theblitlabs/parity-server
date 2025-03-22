#!/bin/bash

# Script to install git hooks for the Parity Runner project

set -e

echo "Setting up git hooks..."

# Ensure our hooks directory exists
mkdir -p scripts/hooks

# Make sure our hook scripts are executable
chmod +x scripts/hooks/pre-commit
chmod +x scripts/hooks/commit-msg

# Install our custom hooks directly
echo "Installing custom hooks..."

# Create backup of any existing hooks
if [ -f ".git/hooks/pre-commit" ]; then
    mv .git/hooks/pre-commit .git/hooks/pre-commit.bak
fi

if [ -f ".git/hooks/commit-msg" ]; then
    mv .git/hooks/commit-msg .git/hooks/commit-msg.bak
fi

# Install our hooks directly
cp scripts/hooks/pre-commit .git/hooks/pre-commit
cp scripts/hooks/commit-msg .git/hooks/commit-msg

# Make the hooks executable
chmod +x .git/hooks/pre-commit
chmod +x .git/hooks/commit-msg

echo "Git hooks successfully installed!" 