# Cloud Client

`cloud.NewDataPlaneClient` connects to `api.safedep.io` with an API key or a workload identity
credential. `cloud.NewControlPlaneClient` connects to `cloud.safedep.io` with a JWT. Both send the
tenant in the `x-tenant-id` header.

## Use in a tool

```go
client, err := cloud.NewDataPlaneClient("pmg", creds, cloud.WithKeepalive())
if err != nil {
	return err
}
defer client.Close()
```

A call without options makes the same connection as before. `cloud.WithDialOptions` passes other
gRPC dial options.

## Rules

- **Dial options are trusted code.** The client does not check them. `grpc.WithAuthority` together
  with `grpc.WithContextDialer` sends the API key to another host over valid TLS. Resolvers,
  interceptors and default call options can also change where and what the client sends. Pass only
  options that the tool itself controls.
- **The client sets TLS and authentication last.** `WithDialOptions` adds the gRPC dial options of
  the caller first. The client then adds its transport credentials and its per-RPC credentials.
- **A caller transport credential does not remove TLS.** gRPC keeps the last transport credentials.
  A caller `grpc.WithTransportCredentials` has no effect.
  `INSECURE_GRPC_CLIENT_USE_INSECURE_TRANSPORT=true` is the only switch for a connection without
  TLS.
- **A caller per-RPC credential cannot replace the authorization or the tenant header.** For one
  header name the last per-RPC credential wins. A caller credential can add a new header. A default
  call option or an interceptor that adds the same header sends a second value after the value of
  the client.
- **A credentials bundle fails the dial after about 10 seconds.** gRPC refuses
  `grpc.WithCredentialsBundle` together with transport credentials, and the adapter retries the
  dial 10 times before it returns the error.
- **Keepalive is off by default.** `drygrpc.KeepaliveDialOption()` pings an idle connection every 30
  seconds and closes it when a ping gets no answer in 10 seconds. Without pings, an RPC on a
  connection that a NAT or a firewall dropped waits until its deadline. A grpc-go server with the
  default enforcement policy closes a connection that pings this often, with GOAWAY
  `too_many_pings`. So a tool turns keepalive on only for a server that permits it. The public SafeDep API
  permits it, because Cloudflare ends HTTP/2 in front of it. A direct connection to a grpc-go
  server, for example in local development or inside a cluster, gets `too_many_pings` and
  reconnects.
