# Contributing to Cortex (cx)

Thanks for your interest in contributing! This document provides guidelines for contributing to the project.

## Getting Started

1. **Fork the repository** and clone your fork
2. **Install Go 1.24+**
3. **Build**: `go build ./cmd/cx`
4. **Run tests**: `go test ./...`

## How to Contribute

### Reporting Bugs

- Check existing issues first to avoid duplicates
- Use a clear, descriptive title
- Include steps to reproduce
- Include your OS, Go version, and cx version (`cx --version`)

### Suggesting Features

- Open an issue describing the feature
- Explain the use case and why it would be valuable
- Be open to discussion about implementation approaches

### Submitting Pull Requests

1. Create a branch from `master`
2. Make your changes
3. Add tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Run `go fmt ./...` to format code
6. Write a clear commit message
7. Open a PR with a description of changes

### Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Keep functions focused and small
- Add comments for non-obvious logic
- Write tests for new features

## Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/cortex.git
cd cortex

# Build
go build ./cmd/cx

# Run tests
go test ./...

# Test your changes
./cx scan /path/to/test/project
./cx find <entity>
```

## Questions?

Open an issue with your question and we'll do our best to help.
