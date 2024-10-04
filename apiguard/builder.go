package apiguard

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
)

const (
	// API Guard config is the source of truth for these
	// headers. Changes should be reflected in the API Guard
	headerRemoteAddr         = "X-Remote-Addr"
	headerPath               = "X-Path"
	headerRequestId          = "X-Request-Id"
	headerTrustToken         = "X-Gateway-Trust"
	headerTokenEmail         = "X-Jwt-Email"
	headerTokenEmailVerified = "X-Jwt-Email-Verified"
	headerTokenSub           = "X-Jwt-Sub"
	headerTokenAud           = "X-Jwt-Aud"
	headerMetaOrgId          = "X-Org-Id"
	headerMetaTeamId         = "X-Team-Id"
	headerMetaUserId         = "X-User-Id"
	headerMetaKeyId          = "X-Key-Id"
)

// SecurelyBuildFromHeader builds a context from the header and validates
// the trust token. Multiple tokens can be passed to allow zero downtime
// token rotation at the API Guard.
func SecurelyBuildFromHeader(header http.Header, tokens ...string) (*Context, error) {
	if len(tokens) == 0 {
		trustToken := os.Getenv("APIGUARD_TRUST_TOKEN")
		if trustToken != "" {
			tokens = append(tokens, trustToken)
		}
	}

	if len(tokens) == 0 {
		return nil, errors.New("APIGuard: Trust token not provided")
	}

	ctx, err := buildFromHeader(header)
	if err != nil {
		return nil, err
	}

	for _, trustToken := range tokens {
		if subtle.ConstantTimeCompare([]byte(ctx.TrustToken), []byte(trustToken)) == 1 {
			return ctx, nil
		}
	}

	return nil, errors.New("APIGuard: Trust token mismatch")
}

// Build a context from the header. This is useful when the API Guard
// is a reverse proxy and the information is passed as headers.
func buildFromHeader(header http.Header) (*Context, error) {
	ctx := Context{
		RemoteAddr: header.Get(headerRemoteAddr),
		RequestID:  header.Get(headerRequestId),
		Path:       header.Get(headerPath),
		TrustToken: header.Get(headerTrustToken),
		Key: KeyInfo{
			OrganizationID: header.Get(headerMetaOrgId),
			TeamID:         header.Get(headerMetaTeamId),
			UserID:         header.Get(headerMetaUserId),
			KeyID:          header.Get(headerMetaKeyId),
		},
		Token: TokenInfo{
			Email:         header.Get(headerTokenEmail),
			EmailVerified: header.Get(headerTokenEmailVerified) == "true",
			Subject:       header.Get(headerTokenSub),
			Audience:      header.Get(headerTokenAud),
		},
	}

	return &ctx, nil
}
