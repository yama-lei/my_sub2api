package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskSiteName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"one char", "a", "**"},
		{"two chars", "ab", "**"},
		{"three chars", "abc", "a**c"},
		{"five chars", "alice", "a**e"},
		{"long name", "rayama114514", "ra**14"},
		{"long email", "alice@example.com", "al**om"},
		{"cjk keeps runes", "张三丰同学", "张**学"},
		{"trims spaces", "  bob  ", "b**b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, MaskSiteName(tc.input))
		})
	}
}

func TestMaskSiteNameNeverReturnsOriginal(t *testing.T) {
	for _, input := range []string{"alice@example.com", "my-prod-key", "单字", "abcdef"} {
		require.NotEqual(t, input, MaskSiteName(input))
	}
}
