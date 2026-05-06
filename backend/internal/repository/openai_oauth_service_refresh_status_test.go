package repository

import (
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestClassifyOpenAIRefreshTokenError_ReusedTokenReturnsUnauthorized(t *testing.T) {
	err := classifyOpenAIRefreshTokenError(http.StatusBadRequest, `{"error":{"message":"Your refresh token has already been used to generate a new access token. Please try signing in again.","code":"refresh_token_reused"}}`)

	require.Equal(t, http.StatusUnauthorized, infraerrors.Code(err))
	require.Equal(t, "OPENAI_OAUTH_TOKEN_REFRESH_FAILED", infraerrors.Reason(err))
	require.Contains(t, err.Error(), "refresh_token_reused")
}
