package modeltrace

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand/v2"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// testdata/parity.json is produced by running the original ModelTrace
// static/fingerprint-core.js under node on real reference outputs.
type parityFixture struct {
	Analyses []struct {
		Name    string `json:"name"`
		Outputs []struct {
			Text          string `json:"text"`
			ExpectedCount int    `json:"expected_count"`
		} `json:"outputs"`
		Expected struct {
			Prediction         string  `json:"prediction"`
			PredictionName     string  `json:"prediction_name"`
			Probability        float64 `json:"probability"`
			UsedOutputs        int     `json:"used_outputs"`
			FamilyPrediction   string  `json:"family_prediction"`
			FamilyProbability  float64 `json:"family_probability"`
			CalibrationQueries string  `json:"calibration_queries"`
			CVAccuracy         float64 `json:"cv_accuracy"`
			Diagnostics        []struct {
				Index          int  `json:"index"`
				ParsedNumbers  int  `json:"parsed_numbers"`
				MinimumNumbers int  `json:"minimum_numbers"`
				Accepted       bool `json:"accepted"`
			} `json:"diagnostics"`
			Results []struct {
				Model                  string  `json:"model"`
				DisplayName            string  `json:"display_name"`
				Family                 string  `json:"family"`
				FamilyName             string  `json:"family_name"`
				Probability            float64 `json:"probability"`
				ConditionalProbability float64 `json:"conditional_probability"`
				ProfileSimilarity      float64 `json:"profile_similarity"`
				Score                  float64 `json:"score"`
			} `json:"results"`
		} `json:"expected"`
	} `json:"analyses"`
	ParseNumbers []struct {
		Text    string `json:"text"`
		Numbers []int  `json:"numbers"`
	} `json:"parse_numbers"`
}

func loadFixture(t *testing.T) parityFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var f parityFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

const tolerance = 1e-9

func near(a, b float64) bool { return math.Abs(a-b) <= tolerance }

func TestAnalyzeParityWithJS(t *testing.T) {
	f := loadFixture(t)
	if len(f.Analyses) == 0 {
		t.Fatal("empty fixture")
	}
	for _, tc := range f.Analyses {
		t.Run(tc.Name, func(t *testing.T) {
			outputs := make([]Output, len(tc.Outputs))
			for i, o := range tc.Outputs {
				outputs[i] = Output{Text: o.Text, ExpectedCount: o.ExpectedCount}
			}
			got, err := Analyze(outputs)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.Expected
			if got.Prediction != want.Prediction || got.PredictionName != want.PredictionName {
				t.Fatalf("prediction = %s, want %s", got.Prediction, want.Prediction)
			}
			if !near(got.Probability, want.Probability) {
				t.Errorf("probability = %.17g, want %.17g", got.Probability, want.Probability)
			}
			if got.UsedOutputs != want.UsedOutputs || got.CalibrationQueries != want.CalibrationQueries || got.CVAccuracy != want.CVAccuracy {
				t.Errorf("used/calibration = %d/%s/%g, want %d/%s/%g", got.UsedOutputs, got.CalibrationQueries, got.CVAccuracy,
					want.UsedOutputs, want.CalibrationQueries, want.CVAccuracy)
			}
			if got.FamilyPrediction != want.FamilyPrediction || !near(got.FamilyProbability, want.FamilyProbability) {
				t.Errorf("family = %s %.17g, want %s %.17g", got.FamilyPrediction, got.FamilyProbability, want.FamilyPrediction, want.FamilyProbability)
			}
			if len(got.Diagnostics) != len(want.Diagnostics) {
				t.Fatalf("diagnostics len = %d, want %d", len(got.Diagnostics), len(want.Diagnostics))
			}
			for i, d := range want.Diagnostics {
				g := got.Diagnostics[i]
				if g.Index != d.Index || g.ParsedNumbers != d.ParsedNumbers || g.MinimumNumbers != d.MinimumNumbers || g.Accepted != d.Accepted {
					t.Errorf("diagnostic[%d] = %+v, want %+v", i, g, d)
				}
			}
			if len(got.Results) != len(want.Results) {
				t.Fatalf("results len = %d, want %d", len(got.Results), len(want.Results))
			}
			for i, w := range want.Results {
				g := got.Results[i]
				if g.Model != w.Model || g.DisplayName != w.DisplayName || g.Family != w.Family || g.FamilyName != w.FamilyName {
					t.Errorf("result[%d] = %s/%s, want %s/%s", i, g.Model, g.Family, w.Model, w.Family)
					continue
				}
				if !near(g.Probability, w.Probability) || !near(g.ConditionalProbability, w.ConditionalProbability) ||
					!near(g.ProfileSimilarity, w.ProfileSimilarity) || !near(g.Score, w.Score) {
					t.Errorf("result[%d] %s = %+v, want %+v", i, g.Model, g, w)
				}
			}
		})
	}
}

func TestParseNumbersParityWithJS(t *testing.T) {
	for _, tc := range loadFixture(t).ParseNumbers {
		got := ParseNumbers(tc.Text)
		if !reflect.DeepEqual(got, tc.Numbers) {
			t.Errorf("ParseNumbers(%q) = %v, want %v", tc.Text, got, tc.Numbers)
		}
	}
}

func TestParseNumbersEdgeCases(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"", []int{}},
		{"no digits", []int{}},
		{"1, 2, 3", []int{1, 2, 3}},
		{"0 356 5 355", []int{5, 355}},                   // out of range dropped, run not split
		{"007 0 000355", []int{7, 355}},                  // leading zeros
		{"1 2 x 3 4 5", []int{3, 4, 5}},                  // letter splits runs; longest wins
		{"1 2 x 3 4", []int{1, 2}},                       // tie keeps the earliest run
		{"1 2 é 3", []int{1, 2}},                         // non-ASCII letters split too
		{"1、2，3。4\n5;6 - 7", []int{1, 2, 3, 4, 5, 6, 7}}, // punctuation does not split
		{"1 2 中 3 4 5", []int{3, 4, 5}},                  // CJK is \p{L}
		{"１２ 3", []int{3}},                               // full-width digits are not \d
		{"123456789012345678901234567890 7", []int{7}},   // no overflow
		{"x 1 2", []int{1, 2}},                           // leading letters with empty run do not split
		{"-3 4.5", []int{3, 4, 5}},
	}
	for _, tc := range cases {
		got := ParseNumbers(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseNumbers(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMinimumNumbersFor(t *testing.T) {
	for expected, want := range map[int]int{0: 80, 100: 80, 145: 80, 146: 81, 292: 161, 332: 183} {
		if got := MinimumNumbersFor(expected); got != want {
			t.Errorf("MinimumNumbersFor(%d) = %d, want %d", expected, got, want)
		}
	}
}

func TestAnalyzeNoAcceptedOutput(t *testing.T) {
	_, err := Analyze([]Output{{Text: "1 2 3", ExpectedCount: 300}})
	if !errors.Is(err, ErrNoAcceptedOutput) {
		t.Fatalf("err = %v, want ErrNoAcceptedOutput", err)
	}
	if _, err := Analyze(nil); !errors.Is(err, ErrNoAcceptedOutput) {
		t.Fatalf("nil outputs: err = %v", err)
	}
}

func TestKnownModelsAndProbabilityOf(t *testing.T) {
	models := KnownModels()
	if len(models) != 18 || models[0] != "gpt-5.4" {
		t.Fatalf("KnownModels = %v", models)
	}
	f := loadFixture(t)
	tc := f.Analyses[0]
	outputs := []Output{}
	for _, o := range tc.Outputs {
		outputs = append(outputs, Output{Text: o.Text, ExpectedCount: o.ExpectedCount})
	}
	a, err := Analyze(outputs)
	if err != nil {
		t.Fatal(err)
	}
	total := 0.0
	for _, m := range models {
		p, ok := ProbabilityOf(a, m)
		if !ok {
			t.Fatalf("ProbabilityOf(%s) missing", m)
		}
		total += p
	}
	if math.Abs(total-1) > 1e-12 {
		t.Errorf("probabilities sum to %g", total)
	}
	if _, ok := ProbabilityOf(a, "nope"); ok {
		t.Error("unknown model reported")
	}
	if _, ok := ProbabilityOf(nil, models[0]); ok {
		t.Error("nil analysis reported")
	}
}

func TestGenerateChallenges(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	cs := GenerateChallengesWithRand(MaxChallenges+10, rng)
	if len(cs) != MaxChallenges {
		t.Fatalf("len = %d, want %d", len(cs), MaxChallenges)
	}
	seen := map[int]bool{}
	ids := map[string]bool{}
	for i, c := range cs {
		if c.ExpectedCount < 292 || c.ExpectedCount > 332 || seen[c.ExpectedCount] {
			t.Fatalf("bad/duplicate length %d", c.ExpectedCount)
		}
		seen[c.ExpectedCount] = true
		prefix := "probe-" + strconv.Itoa(i+1) + "-"
		if !strings.HasPrefix(c.ID, prefix) || len(c.ID) != len(prefix)+14 || ids[c.ID] {
			t.Fatalf("bad id %q", c.ID)
		}
		ids[c.ID] = true
		if !strings.Contains(c.Prompt, " "+strconv.Itoa(c.ExpectedCount)+" 个 1 到 355（含端点）的整数。") ||
			!strings.HasSuffix(c.Prompt, challengeTail) || !strings.Contains(c.Prompt, "禁止调用或借助任何工具") {
			t.Fatalf("unexpected prompt %q", c.Prompt)
		}
	}

	a := GenerateChallengesWithRand(3, rand.New(rand.NewPCG(7, 7)))
	b := GenerateChallengesWithRand(3, rand.New(rand.NewPCG(7, 7)))
	if !reflect.DeepEqual(a, b) {
		t.Error("same seed should give identical challenges")
	}
	if got := GenerateChallenges(0); got != nil {
		t.Errorf("GenerateChallenges(0) = %v", got)
	}
	if got := GenerateChallenges(3); len(got) != 3 {
		t.Errorf("GenerateChallenges(3) len = %d", len(got))
	}
}
