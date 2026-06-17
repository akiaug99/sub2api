package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDeriveUpstreamEndpoint_OpenAICompatibleDomesticPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		inbound  string
		rawPath  string
		platform string
		want     string
	}{
		{"deepseek completions", EndpointChatCompletions, "/v1/chat/completions", service.PlatformDeepSeek, EndpointResponses},
		{"qwen responses compact", EndpointResponses, "/qwen/v1/responses/compact", service.PlatformQwen, "/v1/responses/compact"},
		{"zhipu embeddings", EndpointEmbeddings, "/v1/embeddings", service.PlatformZhipu, EndpointEmbeddings},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, DeriveUpstreamEndpoint(tt.inbound, tt.rawPath, tt.platform))
		})
	}
}
