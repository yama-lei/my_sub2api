package modeltrace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// 挑战模板与上游 fingerprint.generate_challenges 保持一致（中文、第一反应取值）。
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

// Challenge 是一次检测挑战。
type Challenge struct {
	ID            string `json:"id"`
	ExpectedCount int    `json:"expected_count"`
	Prompt        string `json:"prompt"`
}

func cryptoInt(n int) int {
	if n <= 0 {
		return 0
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(value.Int64())
}

func cryptoHex(byteLen int) string {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		for i := range buf {
			buf[i] = byte(cryptoInt(256))
		}
	}
	return hex.EncodeToString(buf)
}

// GenerateChallenges 生成 count 个挑战：数量在 292..332 中不重复采样，
// 提示词由模板随机拼装，要求模型直接凭第一反应输出整数序列。
func GenerateChallenges(count int) ([]Challenge, error) {
	if count <= 0 {
		return nil, fmt.Errorf("modeltrace: 挑战数量必须为正")
	}
	// 与上游 rng.sample(range(292, 333), count) 一致：不重复采样。
	lengths := make([]int, 0, 41)
	for value := 292; value <= 332; value++ {
		lengths = append(lengths, value)
	}
	// Fisher-Yates 随机洗牌后取前 count 个。
	for i := len(lengths) - 1; i > 0; i-- {
		j := cryptoInt(i + 1)
		lengths[i], lengths[j] = lengths[j], lengths[i]
	}
	if count > len(lengths) {
		count = len(lengths)
	}

	challenges := make([]Challenge, 0, count)
	for index := 0; index < count; index++ {
		length := lengths[index]
		var sb strings.Builder
		sb.WriteString(challengeOpenings[cryptoInt(len(challengeOpenings))])
		sb.WriteString("。")
		sb.WriteString(challengeActions[cryptoInt(len(challengeActions))])
		sb.WriteString(fmt.Sprintf(" %d 个 1 到 355（含端点）的整数。", length))
		sb.WriteString("每个位置都要单独选择；不要从 1 开始计数，不要连续递增或递减，也不要采用等差、循环、重复区块或其他规则化模式。")
		sb.WriteString("本任务必须由当前语言模型直接完成：禁止调用或借助任何工具，包括 Python、代码执行器、计算器、搜索、API 和外部随机数生成器；也不要先编写或运行代码。")
		sb.WriteString(challengeEndings[cryptoInt(len(challengeEndings))])
		sb.WriteString(challengeSeparatorHints[cryptoInt(len(challengeSeparatorHints))])
		sb.WriteString("直接从第一个取值开始输出，不要在序列前重复数量、范围或任务说明。")
		challenges = append(challenges, Challenge{
			ID:            fmt.Sprintf("probe-%d-%s", index+1, cryptoHex(7)),
			ExpectedCount: length,
			Prompt:        sb.String(),
		})
	}
	return challenges, nil
}
