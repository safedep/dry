package usefulerror

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func init() {
	registerInternalErrorConverters("grpc/unauthenticated", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
			return NewUsefulError().
				WithCode(ErrAuthenticationFailed).
				WithHumanError("Authentication failed").
				WithHelp("Check your credentials and try again.").
				WithAdditionalHelp("If you are using a token, make sure it is valid and not expired.").
				Wrap(err), true
		}

		return nil, false
	})
}
