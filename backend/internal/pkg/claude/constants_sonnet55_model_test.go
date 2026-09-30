package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelsContainsSonnet55(t *testing.T) {
	for _, model := range DefaultModels {
		if model.ID == "claude-sonnet-5-5" {
			if model.DisplayName != "Claude Sonnet 5.5" || model.CreatedAt != "2026-09-28T00:00:00Z" {
				t.Fatalf("unexpected Sonnet 5.5 descriptor: %+v", model)
			}
			return
		}
	}
	t.Fatal("claude-sonnet-5-5 missing")
}

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
