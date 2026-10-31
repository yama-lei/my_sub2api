package modeltrace

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
)

// Challenge is one fingerprint probe prompt (port of fingerprint.py generate_challenges).
type Challenge struct {
	ID            string
	Prompt        string
	ExpectedCount int
}

const (
	challengeMinLength = 292
	challengeMaxLength = 332 // inclusive
	// MaxChallenges is the most challenges GenerateChallenges can return,
	// since lengths are sampled without replacement.
	MaxChallenges = challengeMaxLength - challengeMinLength + 1
)

var (
	challengeOpenings = []string{
		"这是一次独立的数值选择记录",
		"请完成下面的无语义整数选择任务",
		"执行一次第一反应取值记录",
		"生成一组不承载语义的整数选择",
		"进行一轮快速逐项取值",
	}
	challengeActions = []string{
		"为各个位置分别凭第一反应选择",
		"逐项选择",
		"每次只决定当前一项，共给出",
		"分别凭第一反应给出",
		"逐个直接选择",
	}
	challengeEndings = []string{
		"允许某个数字再次出现；每项写出后不要回头排序、去重或替换。",
		"偶然重复是有效的；不要重新排列或修正已经写出的项目。",
		"相同值可以再次出现；输出过程中不要整理或改写前面的项目。",
		"重复值无需删除；不要筛选、重排或补成某种规律。",
		"不必赋予数字任何含义；已经给出的值保持不变。",
	}
	challengeSeparatorHints = []string{
		"数字之间用逗号或空格分隔均可。",
		"使用一种一致的常见分隔符即可。",
		"可以用逗号、空格或换行分隔。",
		"只要每个整数边界清楚，格式可自行选择。",
	}
)

const (
	challengeRules = "每个位置都要单独选择；不要从 1 开始计数，不要连续递增或递减，也不要采用等差、循环、重复区块或其他规则化模式。" +
		"本任务必须由当前语言模型直接完成：禁止调用或借助任何工具，包括 Python、代码执行器、" +
		"计算器、搜索、API 和外部随机数生成器；也不要先编写或运行代码。"
	challengeTail = "直接从第一个取值开始输出，不要在序列前重复数量、范围或任务说明。"
)

// GenerateChallenges returns n probe prompts with distinct lengths drawn from
// [292, 332], using a cryptographically seeded generator. n is clamped to
// [0, MaxChallenges].
func GenerateChallenges(n int) []Challenge {
	var seed [32]byte
	_, _ = crand.Read(seed[:]) // crypto/rand.Read never returns an error on supported platforms
	return GenerateChallengesWithRand(n, rand.New(rand.NewChaCha8(seed)))
}

// GenerateChallengesWithRand is GenerateChallenges with an injectable random
// source (used for IDs, lengths and phrasing), for deterministic tests.
func GenerateChallengesWithRand(n int, rng *rand.Rand) []Challenge {
	n = max(0, min(n, MaxChallenges))
	if n == 0 {
		return nil
	}
	// Sample n lengths without replacement (partial Fisher-Yates).
	pool := make([]int, MaxChallenges)
	for i := range pool {
		pool[i] = challengeMinLength + i
	}
	for i := 0; i < n; i++ {
		j := i + rng.IntN(len(pool)-i)
		pool[i], pool[j] = pool[j], pool[i]
	}
	pick := func(options []string) string { return options[rng.IntN(len(options))] }

	challenges := make([]Challenge, 0, n)
	for index, length := range pool[:n] {
		opening := pick(challengeOpenings)
		action := pick(challengeActions)
		ending := pick(challengeEndings)
		hint := pick(challengeSeparatorHints)
		prompt := fmt.Sprintf("%s。%s %d 个 1 到 355（含端点）的整数。", opening, action, length) +
			challengeRules + ending + hint + challengeTail

		var token [8]byte
		binary.LittleEndian.PutUint64(token[:], rng.Uint64())
		challenges = append(challenges, Challenge{
			ID:            fmt.Sprintf("probe-%d-%s", index+1, hex.EncodeToString(token[:7])),
			Prompt:        prompt,
			ExpectedCount: length,
		})
	}
	return challenges
}
