# Run the NATS tests

The tests of `events/natsjs` and of the JetStream inbox source need a NATS server
with JetStream. Without `DRY_NATS_TEST_URL` they skip.

```bash
docker run -d --name dry-nats -p 4222:4222 nats:2.12.6-alpine -js
DRY_NATS_TEST_URL=nats://127.0.0.1:4222 go test ./async/... ./events/... -count=1 -race
docker rm -f dry-nats
```

Each test makes its own stream, with a name from the test name and a random
suffix, and its own tenant token. The test deletes the stream at the end. So
tests can share one server, and a server can run tests from many packages at
the same time.
