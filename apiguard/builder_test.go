package apiguard

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurelyBuildFromHeader(t *testing.T) {
	cases := []struct {
		name   string
		header map[string][]string
		tokens []string

		expectedXJwtEmailVerified bool
		expectedXJwtEmail         string
		expectedXJwtSub           string
		expectedXJwtAud           string
		expectedXOrgId            string
		expectedXTeamId           string
		expectedXUserId           string
		expectedXKeyId            string

		err error
	}{
		{
			name:   "no tokens",
			header: map[string][]string{},
			tokens: []string{},
			err:    errors.New("APIGuard: Trust token not provided"),
		},
		{
			name: "token does not match",
			header: map[string][]string{
				headerTrustToken: {"invalid"},
			},
			tokens: []string{"valid"},
			err:    errors.New("APIGuard: Trust token mismatch"),
		},
		{
			name: "token matches",
			header: map[string][]string{
				headerTrustToken: {"valid"},
			},
			tokens: []string{"valid"},
			err:    nil,
		},
		{
			name: "verify context",
			header: map[string][]string{
				headerTrustToken:         {"valid"},
				headerTokenEmail:         {"email"},
				headerTokenEmailVerified: {"true"},
				headerTokenSub:           {"sub"},
				headerTokenAud:           {"aud"},
				headerMetaOrgId:          {"org"},
				headerMetaTeamId:         {"team"},
				headerMetaUserId:         {"user"},
				headerMetaKeyId:          {"key"},
			},
			tokens:                    []string{"valid"},
			expectedXJwtEmail:         "email",
			expectedXJwtEmailVerified: true,
			expectedXJwtSub:           "sub",
			expectedXJwtAud:           "aud",
			expectedXOrgId:            "org",
			expectedXTeamId:           "team",
			expectedXUserId:           "user",
			expectedXKeyId:            "key",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			header := make(map[string][]string)
			for k, v := range test.header {
				header[k] = v
			}

			ctx, err := SecurelyBuildFromHeader(header, test.tokens...)
			if test.err != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.err.Error())
			} else {
				assert.NoError(t, err)

				assert.Equal(t, test.expectedXJwtEmail, ctx.Token.Email)
				assert.Equal(t, test.expectedXJwtEmailVerified, ctx.Token.EmailVerified)
				assert.Equal(t, test.expectedXJwtSub, ctx.Token.Subject)
				assert.Equal(t, test.expectedXJwtAud, ctx.Token.Audience)
				assert.Equal(t, test.expectedXOrgId, ctx.Key.OrganizationID)
				assert.Equal(t, test.expectedXTeamId, ctx.Key.TeamID)
				assert.Equal(t, test.expectedXUserId, ctx.Key.UserID)
				assert.Equal(t, test.expectedXKeyId, ctx.Key.KeyID)
			}
		})
	}
}
