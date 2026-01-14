# Strucured Error Handling Library

## Overview

This library provides a builder pattern for creating richly annotated errors with three orthogonal dimensions of metadata:

1. **Fields**: Key-value pairs for structured logging (compatible with zerolog, zap, log/slog)
2. **Properties**: Machine-readable attributes for control flow (Fault, Retryability, StatusCode)
3. **Messages**: Human-readable context via standard error wrapping

The library extends `github.com/pkg/errors` with these capabilities while maintaining compatibility with Go's standard error interfaces.

## Core Components

### Builder Pattern

```go
// Wrap an existing error with a message and optional format arguments, then add properties and fields
errfs := errors.Fields{
    "method", "GET",
    "duration", time.Now().Sub(startTime),
}

// ... 

return errors.With(err, "failed to call legacy auth service (time: %v)", time.Now()).Set(
    errors.FaultInternal,
    errfs,
    "url", url,
    "username", username,
)

// Create a new error with optional format arguments, then add properties and fields
return errors.WithNew("invalid API key (time: %v)", time.Now()).Set(
    errors.FaultCaller,
    errors.StatusCode(resp.StatusCode),
    errfs,
    "url", url,
    "username", username,
)
```

### Properties

```go
type Fault uint8
const (
    FaultUnknown  Fault = iota  // Zero value
    FaultCaller                 // Client/caller is responsible
    FaultInternal               // Internal system failure
)

type Retryability uint8
const (
    UnknownRetryability Retryability = iota
    Retryable
    NonRetryable
)

type StatusCode int  // HTTP status codes for API responses
```

### Fields

```go
type Fields []any  // Key-value pairs: "key1", value1, "key2", value2, ...

errfs := errors.Fields{
    "url", url,
    "api_key", maskedKey,
    "resp_code", resp.StatusCode,
}
```

### Property Traversal

Property getter functions (`GetFault()`, `GetStatusCode()`, `IsRetryable()`) traverse the entire error chain to find values, enabling flexible incremental enrichment:

```go
// Set fault at inner layer
err1 := errors.WithMetadata(baseErr, errors.FaultInternal)
// Add fields at outer layer
err2 := errors.WithMetadata(err1, "user_id", "123")

// Property remains accessible - getters traverse the chain
errors.GetFault(err2)  // Returns FaultInternal (found in err1)
```

**Traversal Rules**:
- Getters search outer → inner, returning the first non-zero value
- Outer layers can override inner values when explicitly set
- All fields are collected from all layers via `GetFields()`
- Properties set at any layer remain accessible

**Override Example**:
```go
err1 := errors.WithMetadata(baseErr, errors.FaultInternal, errors.StatusCode(500))
err2 := errors.WithMetadata(err1, errors.FaultCaller)  // Override fault

errors.GetFault(err2)       // Returns FaultCaller (outer layer wins)
errors.GetStatusCode(err2)  // Returns 500 (from inner layer)
```

This allows different layers to independently contribute metadata without clobbering each other.

## Example Usage

Imagine we have a legacy authentication service that validates API keys. We want to call this service, handle errors appropriately, and log structured information for observability.

ProxyAuthClient should be somewhat agnostic to how the API layer chooses to handle the metadata that we attach to errors, but should still provide *enough* metadata for the API to do its job well.

```go
func (c *ProxyAuthClient) ValidateAPIKey(apiKey string) (*ProxyAuthResponse, error) {
    url := fmt.Sprintf("%s/v2/api-users/", c.BaseURL)
    maskedKey := apiKey[:4] + "..." + apiKey[len(apiKey)-4:]

    // Create reusable fields for logging
    errfs := errors.Fields{
        "url", url,
        "api_key", maskedKey,
        "key_len", len(apiKey),
    }

    // Log with structured fields
    log.Debug().Fields(errfs.List()).Msg("Calling legacy API")

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        // Internal fault: system failed to create request -- can
        // reliably translate this to an HTTP 500 error
        return nil, errors.With(err, "failed to create request").Set(
            errors.FaultInternal,
            errfs,
            "method", "GET",
            "time", time.Now(),
        )
    }

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        // Internal fault: network/connectivity issue -- can reliably translate
        // this to an HTTP 500 error.  Could also be marked `retryable`, allowing
        // the API layer to assign the appropriate error code + message.
        return nil, errors.With(err, "failed to call legacy auth service").Set(errors.FaultInternal, errfs)
    }
    defer resp.Body.Close()

    // Add another field now that we have it
    errfs.Add("resp_code", resp.StatusCode)

    if resp.StatusCode == http.StatusUnauthorized {
        // Caller fault: provided invalid credentials
        return nil, errors.WithNew("invalid API key").Set(
            errors.FaultCaller,
            errors.StatusCode(resp.StatusCode),
            errfs,
        )
    }

    if resp.StatusCode != http.StatusOK {
        // Internal fault: unexpected response from upstream service
        return nil, errors.WithNew("unexpected status code from legacy service").Set(
            errors.FaultInternal,
            errfs
            errors.StatusCode(resp.StatusCode),
            "username", username,
        )
    }

    var resp ProxyAuthResponse
    if err := json.Unmarshal(bodyBytes, &resp); err != nil {
        // Internal fault: malformed response from upstream
        return nil, errors.With(err, "failed to decode response").Set(errors.FaultInternal, errfs)
    }

    if !resp.AccountIsActive() {
        errfs.Add("user_status", resp.Status)
        // Caller fault: valid key but inactive account
        return nil, errors.WithNew("API key is not valid").Set(errors.FaultCaller, errfs)
    }

    return &resp, nil
}
```

```go
// API layer
func AuthHandler(w http.ResponseWriter, r *http.Request) {
    // ...

    legacyUser, err := legacyClient.ValidateAPIKey(r.Header.Get("x-api-key"))
    if errors.GetFault(err) == errors.FaultCaller {
        // This could be a 400 or 403 depending on API semantics.  We fallback to a
        // generic 403 here if no status code was explicitly set.
        code := errors.GetStatusCode(err)
        if code == 0 {
            code = http.StatusForbidden
        }
        c.JSON(code, models.ErrorResponse{Error: err.Error()})
        c.Abort()
        return

    } else if err != nil {
        // This is an internal fault; log full context with all structured fields that
        // were captured, but leak absolutely no details to the requester
        log.Error().Err(err).Fields(errors.ListFields(err)).Msg("failed to validate API key with legacy service")
        c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Internal server error"})
        c.Abort()
        return
    }

    // ...
}
```


## Value Proposition

### Structured Logging with Fields

**Problem**: Traditional error messages embed context as strings, making it impossible to query, filter, or aggregate errors in production logging systems.

```go
// Traditional approach - context lost in string interpolation
return fmt.Errorf("failed to authenticate user %s at %s: %w", email, url, err)
```

**Solution**: Fields enable structured logging without polluting error messages.

```go
// Fields approach - queryable, filterable, aggregatable
errfs := errors.Fields{
    "email", email,
    "url", url,
    "latency_ms", latency,
}
return errors.With(err, "failed to authenticate user").Set(errors.FaultInternal, errfs)

// Later, in logging/observability:
log.Error().Fields(errors.ListFields(err)).Msg(err.Error())
// Produces: {"level":"error","email":"user@example.com","url":"...","latency_ms":523,...}
```

**Benefits**:
- Query production logs: "Show all auth failures for this email"
- Aggregate metrics: "Average latency by endpoint"
- Alert on patterns: "Spike in errors from specific upstream URL"
- Debug context preserved without verbose messages
- Compatible with zerolog, zap, log/slog, and other structured loggers

### Fault Classification

**Problem**: API servers need to distinguish between client errors (4xx) and server errors (5xx), but Go's `error` interface provides no standard way to convey this.

**Solution**: `Fault` property indicates who is responsible for the error.

```go
// Caller's fault - return 4xx, don't retry, don't page on-call
errors.WithNew("invalid email format").Set(errors.FaultCaller)

// Internal fault - return 5xx, may retry, page on-call if persistent
errors.With(err, "database connection failed").Set(errors.FaultInternal)
```

**Benefits**:
- Middleware can automatically map to correct HTTP status codes
- Monitoring systems differentiate user errors from system failures
- SLO/SLA calculations exclude client errors from uptime metrics
- On-call engineers only paged for internal faults
- Automated retry logic can skip caller faults

### Retryability

**Problem**: Not all errors should be retried. Retrying client errors wastes resources; not retrying transient failures causes unnecessary failures.

**Solution**: Explicit `Retryability` property guides retry logic.  It can be integrated with configurable retry mechanisms, circuit breakers, and job schedulers.

```go
// Transient network error - worth retrying
errors.With(err, "connection timeout").
    Set(errors.Retryable, errors.FaultInternal)

// Validation error - retrying won't help
errors.WithNew("invalid JSON schema").
    Set(errors.NonRetryable, errors.FaultCaller)

// Later in client code:
if errors.IsRetryable(err) {
    time.Sleep(backoff)
    return retry()
}
```

**Benefits**:
- Client libraries automatically retry when appropriate
- Circuit breakers can distinguish temporary from permanent failures
- Background jobs can intelligently reschedule failed tasks
- Reduces unnecessary load on upstream services

### Separation of Concerns in APIs

**Critical concept**: An error has three distinct audiences, each requiring different information:

1. **Logs**: Maximum detail for debugging (Fields + full context)
2. **API Response**: Safe, sanitized messages for clients (StatusCode + public message)
3. **Control Flow**: Properties for logical decisions (Fault + Retryability)

#### Example Separation

```go
// Internal database error
err := errors.With(dbErr, "failed to query users table").
    Set(
        errors.FaultInternal,
        errors.StatusCode(500),
        errors.Fields{
            "query", "SELECT * FROM users WHERE id = $1",
            "params", []any{userId},
            "db_host", dbHost,
            "connection_pool_size", poolSize,
        },
    )

// LOGGED: Full diagnostic context
log.Error().
    Fields(errors.ListFields(err)).
    Str("stack", fmt.Sprintf("%+v", err)).
    Msg(err.Error())
// {"level":"error","msg":"failed to query users table: connection refused",
//  "query":"SELECT...","db_host":"10.0.1.5",...}

// RETURNED TO CLIENT: Sanitized, no internal details
w.WriteHeader(errors.GetStatusCode(err))  // 500
json.NewEncoder(w).Encode(map[string]string{
    "error": "internal server error",  // Generic message, no SQL or IPs
})

// ACTED UPON: Logical decisions
if errors.GetFault(err) == errors.FaultInternal {
    metrics.InternalErrors.Inc()
    alerting.PageOnCall("Database connectivity issue")
}
```

#### Why This Matters

**Security**: Prevents leaking sensitive information (SQL queries, internal IPs, stack traces) to API clients while preserving it for operations teams.

**Operations**: Enables rich debugging without compromising the API contract. Logs contain everything needed to diagnose issues.

**Client Experience**: Provides appropriate status codes and actionable messages without overwhelming clients with internal details.

**Automation**: Properties enable middleware and frameworks to handle errors consistently:
- Log appropriate fields
- Return appropriate status codes
- Make appropriate retry decisions
- Route to appropriate monitoring/alerting

## Integration Patterns

### HTTP Middleware

```go
func ErrorHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        err := next.ServeHTTP(w, r)
        if err != nil {
            // Log with full context
            log.Error().
                Fields(errors.ListFields(err)).
                Str("path", r.URL.Path).
                Msg(err.Error())

            // Respond based on properties
            statusCode := errors.GetStatusCode(err)
            if statusCode == 0 {
                statusCode = 500
            }

            // Return sanitized message
            w.WriteHeader(statusCode)
            json.NewEncoder(w).Encode(map[string]string{
                "error": http.StatusText(statusCode),
            })

            // Metrics/alerting based on fault
            if errors.GetFault(err) == errors.FaultInternal {
                metrics.InternalErrors.Inc()
            }
        }
    })
}
```

### Retry Logic

```go
func CallWithRetry(ctx context.Context, fn func() error) error {
    var lastErr error
    for attempt := 0; attempt < maxRetries; attempt++ {
        lastErr = fn()
        if lastErr == nil {
            return nil
        }

        if !errors.IsRetryable(lastErr) {
            return lastErr  // Don't retry client errors
        }

        select {
        case <-time.After(backoff(attempt)):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return lastErr
}
```

## Comparison to Traditional Approaches

| Approach | Structured Logs | Fault Classification | Retryability | Type Safety |
|----------|----------------|---------------------|--------------|-------------|
| `fmt.Errorf` | No | No | No | No |
| `errors.New` + custom types | Partial | Manual | Manual | Yes |
| This library | Yes | Yes | Yes | Yes |

## Summary

This errors library solves the fundamental problem of error handling in production APIs: errors must serve multiple purposes simultaneously. By separating structured fields (for logging), properties (for control flow), and messages (for humans), it enables:

- Rich observability without security leaks
- Intelligent retry and fault tolerance
- Consistent HTTP API behavior
- Type-safe error handling
- Framework-agnostic integration

The builder pattern makes this ergonomic while maintaining full compatibility with Go's error interfaces.
