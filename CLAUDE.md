# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

**Testing:**
- `go test ./...` - Run all tests 
- `go test -v ./...` - Run tests with verbose output
- `go test ./path/to/package` - Run tests for specific package

**Building and Running:**
- `go build` - Build the module
- `go mod tidy` - Clean up dependencies
- `go vet ./...` - Run Go vet static analysis

## Architecture Overview

This is a Go utilities library (`github.com/brynbellomy/go-utils`) containing reusable components organized into focused packages and root-level utilities.

### Key Packages

**errors/** - Enhanced error handling with HTTP status codes, structured errors with fields, and wrapping utilities. Extends `github.com/pkg/errors` with:
- `StatusCoder` for HTTP-aware errors
- `WithFields()` for structured error context
- `WithCause()` for error chaining

**workerpool/** - Generic worker pool implementation with retry logic and exponential backoff:
- `WorkerPool[T]` with configurable workers
- `Job[T]` with retry attempts and delays
- Batch processing capabilities

**fn/** - Functional programming utilities with generics:
- Map, filter, reduce operations
- Slice utilities (reverse, contains, zip)
- Channel operations with context support

**debugutils/** - Debugging utilities including enhanced RWMutex with caller tracking

### Root Level Utilities

The main package provides utilities for:
- **HTTP request unmarshaling** - Struct tags for headers/query params
- **Data structures** - Sets, ordered maps/sets, sorted maps, tagged unions
- **Concurrency** - Channels, mailbox patterns, shutdown coordination
- **Exponential backoff** with jitter and context support
- **PostgreSQL helpers** - Connection utilities
- **Time/JSON/IO utilities**

### Testing Approach

Tests use `github.com/stretchr/testify` with the `require` package for assertions. Test files follow `*_test.go` naming and use separate test packages (e.g., `package utils_test`).

### Important Patterns

- Heavy use of Go generics for type-safe collections and operations
- Context-aware operations throughout
- Error wrapping with stack traces via `github.com/pkg/errors`
- Defensive programming with proper error handling and resource cleanup