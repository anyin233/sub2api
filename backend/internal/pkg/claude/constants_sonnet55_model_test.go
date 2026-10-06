package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSonnet55AdaptiveThinkingFamily(t *testing.T) {
	require.True(t, IsSonnet55("claude-sonnet-5-5"))
	require.True(t, IsSonnet55("anthropic/claude-sonnet-5-5"))
	require.True(t, IsSonnet55("claude-sonnet-5-5-thinking"))
	require.False(t, IsSonnet55("claude-sonnet-5"))
	require.False(t, IsSonnet55("claude-opus-5-5"))
	require.True(t, RequiresAdaptiveThinking("claude-sonnet-5-5"))
	require.True(t, RequiresAdaptiveThinking("claude-opus-5-5"))
	require.False(t, RequiresAdaptiveThinking("claude-sonnet-5"))
	require.False(t, RequiresAdaptiveThinking("claude-sonnet-4-6"))
}

func TestSonnet55EffortLevels(t *testing.T) {
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, EffortLevelsForModel("claude-sonnet-5-5"))
}
