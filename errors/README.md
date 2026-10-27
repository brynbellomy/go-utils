# Structured Error Handling Library

## Overview

This library annotates errors with three orthogonal dimensions of metadata:

1. **Fields**: Key-value pairs for structured logging (compatible with zerolog, zap, log/slog)
2. **Properties**: Machine-readable attributes for control flow (Kind, Fault, Retryability, StatusCode, and custom properties)
3. **Messages**: Human-readable context via standard error wrapping

It is a drop-in replacement for `github.com/pkg/errors` (`New`, `Errorf`, `Wrap`, `Wrapf`, `WithStack`, `WithMessage`, `WithMessagef`, `Cause`) and re-exports the standard library's `Is`, `As`, `Unwrap`, and `Join`.

## One Error Type

Every constructor returns an `*Error`, either as `error` or as `*Error` for fluent construction. `*Error` implements every interface that the standard library, `github.com/pkg/errors`, `fmt`, and `log/slog` look for (`Unwrap`, `Cause`, `Is`, `As`, `Format`, `StackTrace`, and `LogValue`), so wrapping never hides any of them, no matter how a chain is composed. `compat_test.go` checks this over hundreds of chains mixing this package's constructors with `fmt.Errorf`, `errors.Join`, and `github.com/pkg/errors`.

An `*Error` is immutable. Its methods that return `*Error` return a new node wrapping the receiver, so a stored or shared error is never altered.

## Constructing Errors

`New` and `Wrap` take a constant message followed by items, in the style of `log/slog`:

```go
return errors.Wrap(err, "failed to call legacy auth service",
    errors.ErrServiceUnavailable, // a Kind
    errors.FaultUpstream,         // overrides the Kind's default Fault
    errfs,                        // errors.Fields
    "url", url,                   // a field
    slog.Int("attempt", n),       // a field
)

return errors.New("invalid API key", errors.ErrUnauthorized, "key_len", len(key))
```

Each item is one of:
- an `Option`: `Kind`, `Fault`, `Retryability`, `StatusCode`, `Fields`, or a value returned by `Prop`;
- a `slog.Attr`, appended as a field;
- any other value, appended to the fields as-is, so consecutive `"key", value` items form a field.

Keep messages constant and put variable data in fields: constant messages group cleanly in error trackers and dashboards. `Wrapf`, `Errorf` (with `%w` support), and `WithMessagef` format messages for `github.com/pkg/errors` compatibility.

`Wrap`, `Wrapf`, `WithMessage`, `WithMessagef`, `WithStack`, `WithMetadata`, `Annotate`, and `AddStack` return an untyped nil for a nil or typed-nil input, so their results can always be returned directly as `error`.

### Fluent Construction

`With` and `WithNew` return an `*Error` whose methods chain:

```go
return errors.With(err, "failed to call legacy auth service (attempt %d)", n).
    Set(errors.FaultInternal, errfs, "url", url).
    CausedBy(ctxErr)

return errors.WithNew("invalid API key").Set(errors.ErrUnauthorized)
```

`With` formats its message with `fmt.Sprintf` when given arguments. `Set` accepts the same items as `New` and `Wrap`.

The chain is nil-safe: `With(nil, ...)` returns a nil `*Error`, and every method on a nil `*Error` returns nil. Go boxes a nil `*Error` returned from a function with return type `error` as a non-nil `error`, so end a chain with `.Err()` when the parent error may be nil:

```go
// err may be nil here: .Err() is required
return errors.With(err, "context").Set(errors.FaultInternal).Err()

// err is known non-nil: the *Error can be returned directly
if err != nil {
    return errors.With(err, "context").Set(errors.FaultInternal)
}
```

`Wrap` has no such caveat, since it returns `error`.

## Kinds

A `Kind` classifies an error. Each Kind is a sentinel error, an `Option`, and a set of defaults:

| Kind | `Error()` | Status | Fault | Retryability |
|---|---|---|---|---|
| `ErrBadRequest` | bad request | 400 | caller | unknown |
| `ErrUnauthorized` | unauthorized | 401 | caller | unknown |
| `ErrForbidden` | forbidden | 403 | caller | unknown |
| `ErrNotFound` | not found | 404 | caller | unknown |
| `ErrConflict` | conflict | 409 | caller | unknown |
| `ErrTooManyRequests` | too many requests | 429 | caller | retryable |
| `ErrUnimplemented` | not implemented | 501 | caller | unknown |
| `ErrInternalServerError` | internal server error | 500 | internal | unknown |
| `ErrServiceUnavailable` | service unavailable | 503 | internal | retryable |
| `ErrTimeout` | timeout | 504 | internal | retryable |

```go
// As a sentinel:
return errors.ErrNotFound
return errors.Wrap(errors.ErrNotFound, "load user", "id", id)

// As an option, classifying an existing error:
return errors.Wrap(err, "load user", errors.ErrNotFound, "id", id)

// Either way:
errors.Is(err, errors.ErrNotFound) // true
errors.GetKind(err)                // errors.ErrNotFound
errors.GetStatusCode(err)          // 404
errors.GetFault(err)               // errors.FaultCaller
```

`Kind.String()` returns a stable, space-free name (`"not_found"`) for log fields and metrics labels.

Declare domain sentinels with `Sentinel`, which captures no stack (wrapping a sentinel captures one at the wrap site):

```go
var ErrUserMissing = errors.Sentinel("user missing", errors.ErrNotFound)
```

## Properties

```go
type Fault uint8
const (
    FaultUnknown  Fault = iota  // Zero value
    FaultCaller                 // Client/caller is responsible
    FaultInternal               // Internal system failure
    FaultUpstream               // A relied-upon dependency (RPC node, database, third-party API) failed
)

type Retryability uint8
const (
    UnknownRetryability Retryability = iota
    Retryable
    NonRetryable
)

type StatusCode int  // An explicit HTTP status code, overriding the Kind's default
```

`Fault` and `Retryability` both have a `String()` method returning a lowercase,
stable, space-free name (`"unknown"`, `"caller"`, `"internal"`, `"upstream"` for
Fault; `"unknown"`, `"retryable"`, `"non_retryable"` for Retryability). These are
intended to be used verbatim as structured log field values or Prometheus label
values.

### Custom Properties

`Prop` attaches a value of any type; `Lookup` retrieves the outermost one of a given type. Define a named type per property:

```go
type TenantID string

err := errors.Wrap(err, "load tenant", errors.Prop(TenantID("acme")))
id, ok := errors.Lookup[TenantID](err)
```

`Lookup` matches by assignability, so `T` may be an interface type.

### Property Traversal

`GetKind`, `GetFault`, `GetRetryability`, `IsRetryable`, `GetStatusCode`, and `Lookup` traverse the whole chain, so layers can contribute metadata independently:

```go
err1 := errors.WithMetadata(baseErr, errors.FaultInternal, errors.StatusCode(500))
err2 := errors.WithMetadata(err1, errors.FaultCaller)

errors.GetFault(err2)       // FaultCaller (outer layer wins)
errors.GetStatusCode(err2)  // 500 (from inner layer)
```

**Traversal rules**:
- Getters search outer → inner and return the first layer that sets the property.
- On a single layer, an explicit `Fault`, `Retryability`, or `StatusCode` overrides that layer's `Kind` default; across layers, the outer layer wins, whether its value is explicit or a Kind default.
- An error built with `WithCause` or `CausedBy` is searched through its wrapped error first and its cause second. `errors.Is`, `errors.As`, and `GetFields` reach the cause branch too.
- At a join (`Join`, or anything implementing `Unwrap() []error`):
  - `GetFault` returns the most severe Fault among the children: `FaultInternal`, then `FaultUpstream`, then `FaultCaller`.
  - `GetRetryability` returns `NonRetryable` if any child is NonRetryable, `Retryable` if every child is Retryable, and `UnknownRetryability` otherwise.
  - `GetKind`, `GetStatusCode`, and `Lookup` return the first child's value that is set.
  - `GetFields` concatenates every child's fields.
- An outer layer wrapping a join overrides whatever the join's children carry.

## Fields

```go
type Fields []any  // Key-value pairs: "key1", value1, "key2", value2, ...

errfs := errors.Fields{
    "url", url,
    "api_key", maskedKey,
}
errfs.Add("resp_code", resp.StatusCode)
```

`GetFields(err)` (or `ListFields`) collects every field in the chain, outermost layer first. `LookupField(err, key)` returns the outermost value for a key; only string keys are considered:

```go
if val, ok := errors.LookupField(err, "url"); ok {
    fmt.Println("failed url:", val)
}
```

## Formatting and Logging

- `Error()`, `%s`, and `%v` print the message chain only: `"load user: no rows"`. Fields never appear in them, so wrapping with `fmt.Errorf("...: %w", err)` never bakes fields into a message.
- `%q` prints `Error()` quoted.
- `%+v` prints the message, then every field in logfmt form, then one stack trace:

```
load user: no rows id=7 table=users
main.loadUser
	/app/user.go:42
...
```

A stack is captured once, by the first constructor in a chain that finds no stack below it, so `%+v` prints exactly one trace: the one closest to where the error originated. `StackTrace()` returns that trace in `github.com/pkg/errors` form for error trackers and loggers that look for it.

`*Error` implements `slog.LogValuer`, so `slog.Any("err", err)` logs a structured group:

```go
logger.Error("request failed", "err", err)
// {"msg":"request failed","err":{"msg":"load user: no rows","kind":"not_found","fault":"caller","id":7,"table":"users"}}
```

## Helpers

- `Annotate(&err, msg, args...)` and `AddStack(&err)` wrap a named return value from a `defer`.
- `OneOf(err, targets...)` reports whether `errors.Is(err, target)` holds for any target.
- `AsType[E](err)` is the generic form of `errors.As`.

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
            errfs,
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

A single error type keeps this ergonomic while staying fully compatible with Go's error interfaces and `github.com/pkg/errors`.
