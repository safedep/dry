# Cloud Client

`cloud.NewDataPlaneClient` connects to `api.safedep.io` with an API key or a workload identity
credential. `cloud.NewControlPlaneClient` connects to `cloud.safedep.io` with a JWT. Both send the
tenant in the `x-tenant-id` header.

## Use in a tool

```go
client, err := cloud.NewDataPlaneClient("pmg", creds,
	cloud.WithDialOptions(drygrpc.KeepaliveDialOption()))
if err != nil {
	return err
}
defer client.Close()
```

`drygrpc` is `github.com/safedep/dry/adapters/grpc`. A call without options makes the same
connection as before.

## Rules

- **The client sets TLS and authentication last.** `WithDialOptions` adds the gRPC dial options of
  the caller first. The client then adds its transport credentials and its per-RPC credentials.
- **A conflicting option does not remove TLS.** gRPC keeps the last transport credentials. A caller
  `grpc.WithTransportCredentials` has no effect. `INSECURE_GRPC_CLIENT_USE_INSECURE_TRANSPORT=true`
  is the only switch for a connection without TLS.
- **A caller per-RPC credential cannot replace the authorization or the tenant header.** gRPC sends
  the headers of each per-RPC credential, and for one header name the last credential wins. A caller
  credential can add a new header.
- **A credentials bundle fails the dial.** gRPC refuses `grpc.WithCredentialsBundle` together with
  transport credentials.
- **Keepalive is off by default.** `drygrpc.KeepaliveDialOption()` pings an idle connection every 30
  seconds and closes it when a ping gets no answer in 10 seconds. Without pings, an RPC on a
  connection that a NAT or a firewall dropped waits until its deadline. A grpc-go server with the
  default enforcement policy closes a connection that pings this often, with GOAWAY
  `too_many_pings`. So a tool turns keepalive on only for a server that permits it. The SafeDep API
  permits it, because Cloudflare ends HTTP/2 in front of it.
