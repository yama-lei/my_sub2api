package modeltrace

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func mustLoadTestBank(t *testing.T) *Bank {
	t.Helper()
	bank, err := LoadBank()
	if err != nil {
		t.Fatalf("加载内嵌指纹库失败: %v", err)
	}
	if len(bank.Models) == 0 {
		t.Fatal("指纹库为空")
	}
	if !bank.HasModel("claude-opus-4-8") || !bank.HasModel("gpt-5.6-terra") {
		t.Fatal("指纹库缺少预期模型")
	}
	return bank
}

// TestParseNumbers 验证与上游一致的解析行为：字母分隔断开 run，标点/空白不断开。
func TestParseNumbers(t *testing.T) {
	cases := []struct {
		text string
		want []int
	}{
		{"1, 2, 3", []int{1, 2, 3}},
		{"1 2 3\n4,5", []int{1, 2, 3, 4, 5}},
		{"值12是34", []int{12}}, // 中文字符断开 run：12 与 34 属不同 run，取最先出现的最长 run
		{"abc 300 def 200", []int{300}}, // 字母断开
		{"400, 355, 0, 7", []int{355, 7}}, // 超范围值丢弃但不断开 run
		{"[1,2,3]", []int{1, 2, 3}},
		{"no digits", nil},
	}
	for _, tc := range cases {
		got := ParseNumbers(tc.text)
		if len(got) != len(tc.want) {
			t.Fatalf("ParseNumbers(%q) = %v, want %v", tc.text, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ParseNumbers(%q) = %v, want %v", tc.text, got, tc.want)
			}
		}
	}
}

func approxEqual(a, b float64) bool {
	if a == b {
		return true
	}
	diff := math.Abs(a - b)
	scale := math.Max(math.Max(math.Abs(a), math.Abs(b)), 1e-9)
	return diff/scale < 1e-9
}

type referenceCase struct {
	Name    string `json:"name"`
	Claimed string `json:"claimed"`
	Outputs []struct {
		Text          string `json:"text"`
		ExpectedCount int    `json:"expected_count"`
	} `json:"outputs"`
	Expected struct {
		Prediction         string  `json:"prediction"`
		PredictionName     string  `json:"prediction_name"`
		Probability        float64 `json:"probability"`
		UsedOutputs        int     `json:"used_outputs"`
		Calibration        struct {
			Queries    string  `json:"queries"`
			Beta       float64 `json:"beta"`
			CVAccuracy float64 `json:"cv_accuracy"`
		} `json:"calibration"`
		FamilyPrediction   string  `json:"family_prediction"`
		FamilyProbability  float64 `json:"family_probability"`
		Top3               []struct {
			Model                  string  `json:"model"`
			Probability            float64 `json:"probability"`
			ProfileSimilarity      float64 `json:"profile_similarity"`
			Score                  float64 `json:"score"`
			ConditionalProbability float64 `json:"conditional_probability"`
		} `json:"top3"`
		FamilyProbabilities []struct {
			Family      string  `json:"family"`
			DisplayName string  `json:"display_name"`
			Probability float64 `json:"probability"`
		} `json:"family_probabilities"`
	} `json:"expected"`
}

// TestAnalyzeAgainstJSReference 用上游 JS 实现在真实参考回答上产生的结果
// 交叉验证 Go 移植（相对误差 < 1e-9）。
func TestAnalyzeAgainstJSReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatalf("读取参考数据失败: %v", err)
	}
	var cases []referenceCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("解析参考数据失败: %v", err)
	}
	bank := mustLoadTestBank(t)

	for _, tc := range cases {
		outputs := make([]Output, 0, len(tc.Outputs))
		for _, item := range tc.Outputs {
			outputs = append(outputs, Output{Text: item.Text, ExpectedCount: item.ExpectedCount})
		}
		attr, err := AnalyzeOutputs(outputs, bank)
		if err != nil {
			t.Fatalf("%s: 归因失败: %v", tc.Name, err)
		}
		if attr.Prediction != tc.Expected.Prediction {
			t.Errorf("%s: prediction = %q, want %q", tc.Name, attr.Prediction, tc.Expected.Prediction)
		}
		if attr.UsedOutputs != tc.Expected.UsedOutputs {
			t.Errorf("%s: used_outputs = %d, want %d", tc.Name, attr.UsedOutputs, tc.Expected.UsedOutputs)
		}
		if attr.FamilyPrediction != tc.Expected.FamilyPrediction {
			t.Errorf("%s: family_prediction = %q, want %q", tc.Name, attr.FamilyPrediction, tc.Expected.FamilyPrediction)
		}
		if !approxEqual(attr.Probability, tc.Expected.Probability) {
			t.Errorf("%s: probability = %.12f, want %.12f", tc.Name, attr.Probability, tc.Expected.Probability)
		}
		if !approxEqual(attr.CalibrationBeta, tc.Expected.Calibration.Beta) {
			t.Errorf("%s: beta = %.12f, want %.12f", tc.Name, attr.CalibrationBeta, tc.Expected.Calibration.Beta)
		}
		if !approxEqual(attr.FamilyProbability, tc.Expected.FamilyProbability) {
			t.Errorf("%s: family_probability = %.12f, want %.12f", tc.Name, attr.FamilyProbability, tc.Expected.FamilyProbability)
		}
		if len(attr.Results) != len(bank.Models) {
			t.Fatalf("%s: results 数量 %d, want %d", tc.Name, len(attr.Results), len(bank.Models))
		}
		for i, want := range tc.Expected.Top3 {
			got := attr.Results[i]
			if got.Model != want.Model {
				t.Errorf("%s: top%d model = %q, want %q", tc.Name, i+1, got.Model, want.Model)
				continue
			}
			if !approxEqual(got.Probability, want.Probability) {
				t.Errorf("%s: top%d(%s) probability = %.12f, want %.12f", tc.Name, i+1, got.Model, got.Probability, want.Probability)
			}
			if !approxEqual(got.ProfileSimilarity, want.ProfileSimilarity) {
				t.Errorf("%s: top%d(%s) profile_similarity = %.12f, want %.12f", tc.Name, i+1, got.Model, got.ProfileSimilarity, want.ProfileSimilarity)
			}
			if !approxEqual(got.Score, want.Score) {
				t.Errorf("%s: top%d(%s) score = %.12f, want %.12f", tc.Name, i+1, got.Model, got.Score, want.Score)
			}
			if !approxEqual(got.ConditionalProbability, want.ConditionalProbability) {
				t.Errorf("%s: top%d(%s) conditional_probability = %.12f, want %.12f", tc.Name, i+1, got.Model, got.ConditionalProbability, want.ConditionalProbability)
			}
		}
		if len(tc.Expected.FamilyProbabilities) == len(attr.FamilyProbabilities) {
			for i, want := range tc.Expected.FamilyProbabilities {
				got := attr.FamilyProbabilities[i]
				if got.Family != want.Family || !approxEqual(got.Probability, want.Probability) {
					t.Errorf("%s: family[%d] = %+v, want %+v", tc.Name, i, got, want)
				}
			}
		}
	}
}

func TestGenerateChallenges(t *testing.T) {
	challenges, err := GenerateChallenges(6)
	if err != nil {
		t.Fatalf("生成挑战失败: %v", err)
	}
	if len(challenges) != 6 {
		t.Fatalf("挑战数量 = %d, want 6", len(challenges))
	}
	seen := map[int]bool{}
	for i, c := range challenges {
		if c.ExpectedCount < 292 || c.ExpectedCount > 332 {
			t.Errorf("挑战 %d 数量越界: %d", i, c.ExpectedCount)
		}
		if seen[c.ExpectedCount] {
			t.Errorf("挑战数量重复采样: %d", c.ExpectedCount)
		}
		seen[c.ExpectedCount] = true
		if len(c.ID) == 0 || len(c.Prompt) < 100 {
			t.Errorf("挑战 %d 字段异常: id=%q prompt_len=%d", i, c.ID, len(c.Prompt))
		}
		if !contains(c.Prompt, "1 到 355") || !contains(c.Prompt, "禁止调用") {
			t.Errorf("挑战 %d 提示词缺少关键指令", i)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestMinimumForExpected(t *testing.T) {
	cases := []struct {
		expected int
		want     int
	}{
		{0, 80},
		{100, 80},
		{145, 80}, // ceil(145*0.55)=80
		{146, 81},
		{292, 161},
		{330, 182},
	}
	for _, tc := range cases {
		if got := MinimumForExpected(tc.expected); got != tc.want {
			t.Errorf("MinimumForExpected(%d) = %d, want %d", tc.expected, got, tc.want)
		}
	}
}

func TestCalibrationBeta(t *testing.T) {
	bank := mustLoadTestBank(t)
	for _, n := range []int{1, 2, 3, 5} {
		beta, _, err := bank.CalibrationBeta(n)
		if err != nil {
			t.Fatalf("CalibrationBeta(%d) 失败: %v", n, err)
		}
		if beta <= 0 {
			t.Errorf("CalibrationBeta(%d) = %v, 应为正", n, beta)
		}
	}
	if _, _, err := bank.CalibrationBeta(0); err == nil {
		t.Error("CalibrationBeta(0) 应返回错误")
	}
}
