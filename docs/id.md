# id

Package `id` makes short, prefixed, random ids, such as the rule id `r_9c1x4kq7dm`.

```go
import "github.com/safedep/dry/id"

ruleID, err := id.New("r")         // "r_" and 10 random characters
scanID, err := id.NewN("scan", 16) // "scan_" and 16 random characters
ok := id.Valid("r", ruleID)
```

## Rules

- An id is the prefix, `_` and random characters.
- The random characters come from `crypto/rand`, through `crypto.RandomString`.
- The alphabet is lowercase Crockford base32: `0123456789abcdefghjkmnpqrstvwxyz`. It has no `i`, `l`,
  `o` or `u`, so a person can read an id aloud and type it.
- `New` makes 10 characters, which give 50 bits. `NewN` accepts 8 to 64 characters. Fewer than 8
  characters make an id that a caller can guess.
- A prefix is 1 to 16 lowercase ASCII letters. It has no digit and no `_`, so the first `_` of an id
  always ends the prefix.
- `Valid` checks the form: the prefix, `_`, and 8 to 64 characters of the alphabet.
- The package does not check for a collision. The caller checks the id against its own store.

`utils.NewUniqueId` makes a ULID from `math/rand`. Do not use it for an id that must be hard to guess.
