//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// OMP (oh-my-pi) server-side compaction 兼容：mimic 路径窄口径保留 compact beta。
func TestCompactionBetaOAuthMimic(t *testing.T) {
	const onDemand = claude.BetaCompaction     // compact-2026-09-04
	const legacy = claude.BetaCompactionLegacy // compact-2026-01-12
	for _, tc := range []struct {
		name   string
		header string
		drop   map[string]struct{}
		want   bool
	}{{"explicit on-demand", onDemand, nil, true},
		{"explicit legacy", legacy, nil, true},
		{"both", onDemand + "," + legacy, nil, true},
		{"mixed with unknown", "custom-beta," + onDemand, nil, true},
		{"absent", "custom-beta", nil, false},
		{"similar token", onDemand + "-other", nil, false},
		{"policy filtered", onDemand, map[string]struct{}{onDemand: {}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestGatewayServiceForBeta(false)
			header := http.Header{}
			header.Set("anthropic-beta", tc.header)
			body := []byte(`{"compaction":{"type":"summarize"}}`)
			got, set := s.computeFinalAnthropicBeta("oauth", true, "claude-sonnet-4-6", header, body, tc.drop)
			require.True(t, set)
			// 用例声明"哪个 token 应被保留"——显式请求的 token 必须出现在结果里
			wantToken := onDemand
			if tc.name == "explicit legacy" || (tc.name == "absent" || tc.name == "similar token") {
				if tc.name == "explicit legacy" {
					wantToken = legacy
				}
			}
			require.Equal(t, tc.want, containsBetaToken(got, wantToken))
			// 未知客户端 beta 仍然被丢弃，CC 指纹完整保留
			require.False(t, containsBetaToken(got, "custom-beta"))
			require.True(t, containsBetaToken(got, claude.BetaOAuth))
			require.True(t, containsBetaToken(got, claude.BetaClaudeCode))
			if tc.want {
				require.Equal(t, 1, countOccurrences(got, wantToken))
			}
		})
	}
}

// mimic 分支的 compact 保留只认 messages 路径的语义；非 mimic OAuth（真 CC 客户端）
// 由透传路径负责，不在此覆盖。
func TestCompactionBetaOAuthPassthroughUnaffected(t *testing.T) {
	s := newTestGatewayServiceForBeta(false)
	hdr := http.Header{}
	hdr.Set("anthropic-beta", claude.BetaCompaction+","+claude.BetaClaudeCode)
	final, ok := s.computeFinalAnthropicBeta("oauth", false, "claude-sonnet-4-6", hdr, []byte(`{}`), nil)
	require.True(t, ok)
	require.True(t, containsBetaToken(final, claude.BetaCompaction))
	require.True(t, containsBetaToken(final, claude.BetaClaudeCode))
}

// body.compaction 的能力维度 sanitize：缺 compact-2026-09-04 时剥掉参数，
// 使 header/body 保持对称（上游不再 400，而是安静地不压缩）。
func TestSanitizeAnthropicBodyForBetaTokens_Compaction(t *testing.T) {
	t.Run("stripped when beta missing", func(t *testing.T) {
		body := []byte(`{"model":"claude-sonnet-4-6","compaction":{"type":"summarize"},"messages":[]}`)
		out, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20,claude-code-20250219")
		require.True(t, changed)
		require.False(t, gjson.GetBytes(out, "compaction").Exists())
	})
	t.Run("kept when beta present", func(t *testing.T) {
		body := []byte(`{"model":"claude-sonnet-4-6","compaction":{"type":"summarize"},"messages":[]}`)
		out, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20,"+claude.BetaCompaction)
		require.False(t, changed)
		require.Equal(t, "summarize", gjson.GetBytes(out, "compaction.type").String())
	})
	t.Run("no field no change", func(t *testing.T) {
		body := []byte(`{"model":"claude-sonnet-4-6","messages":[]}`)
		out, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20")
		require.False(t, changed)
		require.Equal(t, string(body), string(out))
	})
}

// 端到端：mimic 路径 + 客户端带 compact beta → 最终请求头含 compact token、
// body.compaction 保留；policy filter 命中时 token 被剥、body 参数同步剥离。
func TestBuildUpstreamRequestCompactionBeta(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserved", true: "policy filtered"}[filtered], func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			c.Request.Header.Set("anthropic-beta", claude.BetaCompaction+",custom-beta")
			if filtered {
				c.Set(betaPolicyFilterSetKey, map[string]struct{}{claude.BetaCompaction: {}})
			}
			account := &Account{ID: 702, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
				Credentials: map[string]any{"access_token": "test-token"}, Status: StatusActive, Schedulable: true}
			body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":64,"compaction":{"type":"summarize"},"messages":[{"role":"user","content":"pong"}]}`)
			svc := &GatewayService{cfg: &config.Config{}}
			req, _, err := svc.buildUpstreamRequest(context.Background(), c, account, body,
				"test-token", "oauth", "claude-sonnet-4-6", false, true)
			require.NoError(t, err)
			out := readUpstreamBodyForTest(t, req)
			header := getHeaderRaw(req.Header, "anthropic-beta")
			require.Equal(t, !filtered, containsBetaToken(header, claude.BetaCompaction))
			require.False(t, containsBetaToken(header, "custom-beta"))
			require.Equal(t, !filtered, gjson.GetBytes(out, "compaction").Exists())
		})
	}
}

func countOccurrences(s, sub string) int {
	count := 0
	for i := 0; i+len(sub) <= len(s); {
		idx := indexOf(s[i:], sub)
		if idx < 0 {
			break
		}
		count++
		i += idx + len(sub)
	}
	return count
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
