# Streams

General guidelines for using Streams

- Treat a `Stream` as a first class citizen with service classification
- Treat `Stream` as a value and pass along avoiding side-effects
- `Stream` can be multi-tenant in which case `TenantID` is injected before appending
- Use `Stream` methods to create new streams with different tenant IDs instead of mutating the existing stream

## Config

Environment based configuration is supported with `NewDefaultS2StreamProviderConfig`.

## Example

Start by declaring a stream:

```go
stream := stream.Stream{
	Namespace: "my-namespace",
	Name:      "my-stream",
}
```

Create a writer:

```go
writer, _ := stream.NewS2StreamWriter(..., stream, ...)
```

Write a record:

```go
writer.AppendOne(ctx, msg)
```
