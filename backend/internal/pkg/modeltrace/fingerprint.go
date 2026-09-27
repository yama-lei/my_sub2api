package modeltrace

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"unicode"
)

// numberPattern 与上游 re.finditer(r"\d+", text) 一致。
var numberPattern = regexp.MustCompile(`[0-9]+`)

// ParseNumbers 提取文本中最长的一段「1..355 整数序列」：
// 数字段之间若隔了字母则视为不同 run；其余分隔符（逗号/空格/换行/标点）不断开。
// 与上游 fingerprint.parse_numbers 行为一致。
func ParseNumbers(text string) []int {
	var runs [][]int
	var current []int
	previousEnd := 0
	for _, loc := range numberPattern.FindAllStringIndex(text, -1) {
		separator := text[previousEnd:loc[0]]
		value, _ := strconv.Atoi(text[loc[0]:loc[1]])
		if len(current) > 0 && containsLetter(separator) {
			runs = append(runs, current)
			current = nil
		}
		if value >= ValueMin && value <= ValueMax {
			current = append(current, value)
		}
		previousEnd = loc[1]
	}
	if len(current) > 0 {
		runs = append(runs, current)
	}
	var best []int
	for _, run := range runs {
		if len(run) > len(best) {
			best = run
		}
	}
	return best
}

// containsLetter 与 JS /\p{L}/u 及 Python str.isalpha 判定保持一致（按 Unicode 字母）。
func containsLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// CountNumbers 统计数字直方图（维度 355）。
func CountNumbers(numbers []int) []int {
	counts := make([]int, Dimension)
	for _, number := range numbers {
		counts[number-ValueMin]++
	}
	return counts
}

// standardize 人口方差标准化（与上游 standardize 一致，方差下限 1e-12）。
func standardize(values []float64) []float64 {
	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, value := range values {
		diff := value - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	scale := math.Max(math.Sqrt(variance), 1e-12)
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = (value - mean) / scale
	}
	return out
}

func dot(left, right []float64) float64 {
	total := 0.0
	for i := range left {
		total += left[i] * right[i]
	}
	return total
}

func normalize(values []float64) []float64 {
	scale := math.Max(math.Sqrt(dot(values, values)), 1e-12)
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = value / scale
	}
	return out
}

// subtractBasis 投影掉干扰方向（basis 每行一个正交方向）。
// 与 Python `(projected @ basis.T) @ basis` 一致：同时投影，而非逐个消去。
func subtractBasis(values []float64, basis [][]float64) []float64 {
	if len(basis) == 0 {
		return values
	}
	coeffs := make([]float64, len(basis))
	for k, vector := range basis {
		coeffs[k] = dot(values, vector)
	}
	out := make([]float64, len(values))
	copy(out, values)
	for k, vector := range basis {
		for i := range out {
			out[i] -= coeffs[k] * vector[i]
		}
	}
	return out
}

// HellingerFeature counts+Alpha 归一化后开方。
func HellingerFeature(counts []int) []float64 {
	total := float64(0)
	for _, value := range counts {
		total += float64(value)
	}
	total += Alpha * Dimension
	feature := make([]float64, len(counts))
	for i, value := range counts {
		feature[i] = math.Sqrt((float64(value) + Alpha) / total)
	}
	return feature
}

// splitIntoFour 与 numpy.array_split(values, 4) 一致：前 r 块多一个元素。
func splitIntoFour(values []int) [][]int {
	base := len(values) / 4
	remainder := len(values) % 4
	chunks := make([][]int, 0, 4)
	start := 0
	for i := 0; i < 4; i++ {
		size := base
		if i < remainder {
			size++
		}
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
		start = end
	}
	return chunks
}

// OrderedBlockFeature 有序块特征：4 段 × 16 直方图 + 个位分布，共 74 维。
func OrderedBlockFeature(numbers []int) []float64 {
	pieces := make([]float64, 0, 74)
	for _, chunk := range splitIntoFour(numbers) {
		bins := make([]float64, 16)
		for i := range bins {
			bins[i] = 0.5
		}
		for _, value := range chunk {
			index := int(((float64(value) - 1.0) / 355.0) * 16.0)
			if index > 15 {
				index = 15
			}
			bins[index]++
		}
		total := 0.0
		for _, value := range bins {
			total += value
		}
		for _, value := range bins {
			pieces = append(pieces, math.Sqrt(value/total))
		}
	}
	lastDigits := make([]float64, 10)
	for i := range lastDigits {
		lastDigits[i] = 0.5
	}
	for _, value := range numbers {
		lastDigits[value%10]++
	}
	lastTotal := 0.0
	for _, value := range lastDigits {
		lastTotal += value
	}
	for _, value := range lastDigits {
		pieces = append(pieces, math.Sqrt(value/lastTotal))
	}
	return pieces
}

// robustScoreCounts 边际 Hellinger 特征的全局模型得分（标准化 z 分数）。
func robustScoreCounts(counts []int, bank *Bank) []float64 {
	artifact := &bank.Robust.Hellinger
	feature := HellingerFeature(counts)
	projected := make([]float64, len(feature))
	for i, value := range feature {
		projected[i] = (value - artifact.FeatureMean[i]) / artifact.FeatureScale[i]
	}
	projected = normalize(subtractBasis(projected, artifact.NuisanceBasis))
	scores := make([]float64, len(artifact.Centroids))
	for i, centroid := range artifact.Centroids {
		scores[i] = dot(projected, centroid)
	}
	return standardize(scores)
}

// orderedBlockScores 有序块特征的全局模型得分（标准化 z 分数）。
func orderedBlockScores(numbers []int, bank *Bank) []float64 {
	artifact := bank.Robust.OrderedBlocks
	feature := OrderedBlockFeature(numbers)
	standardizedFeature := make([]float64, len(feature))
	for i, value := range feature {
		standardizedFeature[i] = (value - artifact.FeatureMean[i]) / artifact.FeatureScale[i]
	}
	unit := normalize(standardizedFeature)

	// 每个模型取「与 12 个环境模板的最大相似度」，再标准化。
	template := make([]float64, len(artifact.Centroids))
	for modelIndex := range template {
		best := math.Inf(-1)
		for _, environment := range artifact.EnvironmentCentroids {
			if score := dot(unit, environment[modelIndex]); score > best {
				best = score
			}
		}
		template[modelIndex] = best
	}
	template = standardize(template)

	projected := normalize(subtractBasis(standardizedFeature, artifact.NuisanceBasis))
	nuisance := make([]float64, len(artifact.Centroids))
	for i, centroid := range artifact.Centroids {
		nuisance[i] = dot(projected, centroid)
	}
	nuisance = standardize(nuisance)

	fused := make([]float64, len(template))
	for i := range fused {
		fused[i] = 0.5*template[i] + 0.5*nuisance[i]
	}
	return standardize(fused)
}

// RobustScoreNumbers 融合边际与有序块得分：0.75 × 边际 + weight × 有序块。
func RobustScoreNumbers(numbers []int, bank *Bank) []float64 {
	marginal := robustScoreCounts(CountNumbers(numbers), bank)
	artifact := bank.Robust.OrderedBlocks
	if artifact == nil || artifact.Weight == 0 {
		return marginal
	}
	ordered := orderedBlockScores(numbers, bank)
	fused := make([]float64, len(marginal))
	for i := range fused {
		fused[i] = (1.0-artifact.Weight)*marginal[i] + artifact.Weight*ordered[i]
	}
	return fused
}

// Softmax 数值稳定的 softmax。
func Softmax(values []float64) []float64 {
	maximum := math.Inf(-1)
	for _, value := range values {
		if value > maximum {
			maximum = value
		}
	}
	weights := make([]float64, len(values))
	total := 0.0
	for i, value := range values {
		weights[i] = math.Exp(value - maximum)
		total += weights[i]
	}
	for i := range weights {
		weights[i] /= total
	}
	return weights
}

// JSSimilarity 与指纹库直方图的 Jensen-Shannon 相似度（0..1）。
func JSSimilarity(left []int, right []int) float64 {
	leftTotal := 0
	for _, value := range left {
		leftTotal += value
	}
	rightTotal := float64(Alpha * Dimension)
	for _, value := range right {
		rightTotal += float64(value)
	}
	p := make([]float64, len(left))
	q := make([]float64, len(right))
	midpoint := make([]float64, len(left))
	for i := range left {
		p[i] = float64(left[i]) / float64(leftTotal)
		q[i] = (float64(right[i]) + Alpha) / rightTotal
		midpoint[i] = (p[i] + q[i]) / 2.0
	}
	divergence := func(values []float64) float64 {
		total := 0.0
		for i, value := range values {
			if value > 0 {
				total += value * math.Log(value/midpoint[i])
			}
		}
		return total
	}
	js := (divergence(p) + divergence(q)) / 2.0
	return 1.0 - math.Sqrt(js/math.Log(2.0))
}

// ModelResult 是单个候选模型的归因结果。
type ModelResult struct {
	Model                 string  `json:"model"`
	DisplayName           string  `json:"display_name"`
	Family                string  `json:"family"`
	FamilyName            string  `json:"family_name"`
	Probability           float64 `json:"probability"`
	ConditionalProbability float64 `json:"conditional_probability"`
	ProfileSimilarity     float64 `json:"profile_similarity"`
	Score                 float64 `json:"score"`
}

// FamilyResult 是家族聚合概率。
type FamilyResult struct {
	Family      string  `json:"family"`
	DisplayName string  `json:"display_name"`
	Probability float64 `json:"probability"`
}

// Diagnostic 是单份回答的解析诊断。
type Diagnostic struct {
	Index           int  `json:"index"`
	ParsedNumbers   int  `json:"parsed_numbers"`
	MinimumNumbers  int  `json:"minimum_numbers"`
	Accepted        bool `json:"accepted"`
}

// Attribution 是一次完整归因的结果。
type Attribution struct {
	Prediction           string         `json:"prediction"`
	PredictionName       string         `json:"prediction_name"`
	Probability          float64        `json:"probability"`
	UsedOutputs          int            `json:"used_outputs"`
	Results              []ModelResult  `json:"results"`
	Diagnostics          []Diagnostic   `json:"diagnostics"`
	CalibrationBeta      float64        `json:"calibration_beta"`
	CalibrationCVAccuracy float64       `json:"calibration_cv_accuracy"`
	CalibrationQueries   int            `json:"calibration_queries"`
	FamilyPrediction     string         `json:"family_prediction"`
	FamilyPredictionName string         `json:"family_prediction_name"`
	FamilyProbability    float64        `json:"family_probability"`
	FamilyProbabilities  []FamilyResult `json:"family_probabilities"`
	Method               string         `json:"method"`
}

// Output 是一份待归因回答：模型完整输出文本 + 挑战要求的数量。
type Output struct {
	Text          string `json:"text"`
	ExpectedCount int    `json:"expected_count"`
}

// MinValidNumbers 是回答被接受所需的最少数字个数下限。
const MinValidNumbers = 80

// MinimumForExpected 计算某挑战要求下回答被接受的最少数字数。
func MinimumForExpected(expected int) int {
	if expected <= 0 {
		return MinValidNumbers
	}
	minimum := int(math.Ceil(float64(expected) * 0.55))
	if minimum < MinValidNumbers {
		return MinValidNumbers
	}
	return minimum
}

// AnalyzeOutputs 对若干份回答执行统一全局稳健数字指纹归因。
// 迁移自上游 analyze_outputs + analyze_global_outputs。
func AnalyzeOutputs(outputs []Output, bank *Bank) (*Attribution, error) {
	validNumbers := make([][]int, 0, len(outputs))
	validCounts := make([][]int, 0, len(outputs))
	validScores := make([][]float64, 0, len(outputs))
	diagnostics := make([]Diagnostic, 0, len(outputs))

	for index, item := range outputs {
		numbers := ParseNumbers(item.Text)
		minimum := MinimumForExpected(item.ExpectedCount)
		accepted := len(numbers) >= minimum
		diagnostics = append(diagnostics, Diagnostic{
			Index:          index,
			ParsedNumbers:  len(numbers),
			MinimumNumbers: minimum,
			Accepted:       accepted,
		})
		if accepted {
			validNumbers = append(validNumbers, numbers)
			validCounts = append(validCounts, CountNumbers(numbers))
			validScores = append(validScores, RobustScoreNumbers(numbers, bank))
		}
	}

	if len(validScores) == 0 {
		return nil, fmt.Errorf("没有可用回答：请粘贴完整数字序列；拒答或严重截断的回答不会计入")
	}

	modelCount := len(bank.Models)
	combinedScores := make([]float64, modelCount)
	for _, scores := range validScores {
		for i := 0; i < modelCount; i++ {
			combinedScores[i] += scores[i]
		}
	}
	for i := 0; i < modelCount; i++ {
		combinedScores[i] /= float64(len(validScores))
	}

	queries := len(validScores)
	if queries > 3 {
		queries = 3
	}
	beta, cvAccuracy, err := bank.CalibrationBeta(queries)
	if err != nil {
		return nil, err
	}
	scaledScores := make([]float64, modelCount)
	for i := range combinedScores {
		scaledScores[i] = beta * combinedScores[i]
	}
	probabilities := Softmax(scaledScores)

	pooledCounts := make([]int, Dimension)
	for _, counts := range validCounts {
		for i, value := range counts {
			pooledCounts[i] += value
		}
	}

	// 家族聚合。
	familyOrder := make([]string, 0, 2)
	familySeen := make(map[string]bool)
	familyNames := make(map[string]string)
	for i := range bank.Models {
		family := bank.Models[i].Family
		if family == "" {
			family = "models"
		}
		if !familySeen[family] {
			familySeen[family] = true
			familyOrder = append(familyOrder, family)
		}
		if familyNames[family] == "" && bank.Models[i].FamilyName != "" {
			familyNames[family] = bank.Models[i].FamilyName
		}
	}

	results := make([]ModelResult, 0, modelCount)
	for i := range bank.Models {
		family := bank.Models[i].Family
		if family == "" {
			family = "models"
		}
		familyName := familyNames[family]
		if familyName == "" {
			familyName = family
		}
		results = append(results, ModelResult{
			Model:             bank.Models[i].ID,
			DisplayName:       bank.Models[i].DisplayName,
			Family:            family,
			FamilyName:        familyName,
			Probability:       probabilities[i],
			ProfileSimilarity: JSSimilarity(pooledCounts, bank.Models[i].Counts),
			Score:             combinedScores[i],
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Probability > results[j].Probability
	})

	familyProbabilities := make(map[string]float64, len(familyOrder))
	for _, family := range familyOrder {
		total := 0.0
		for i := range results {
			if results[i].Family == family {
				total += results[i].Probability
			}
		}
		familyProbabilities[family] = total
	}
	for i := range results {
		results[i].ConditionalProbability = results[i].Probability / familyProbabilities[results[i].Family]
	}

	winningFamily := familyOrder[0]
	for _, family := range familyOrder {
		if familyProbabilities[family] > familyProbabilities[winningFamily] {
			winningFamily = family
		}
	}

	familyResults := make([]FamilyResult, 0, len(familyOrder))
	for _, family := range familyOrder {
		name := familyNames[family]
		if name == "" {
			name = family
		}
		familyResults = append(familyResults, FamilyResult{
			Family:      family,
			DisplayName: name,
			Probability: familyProbabilities[family],
		})
	}

	return &Attribution{
		Prediction:            results[0].Model,
		PredictionName:        results[0].DisplayName,
		Probability:           results[0].Probability,
		UsedOutputs:           len(validScores),
		Results:               results,
		Diagnostics:           diagnostics,
		CalibrationBeta:       beta,
		CalibrationCVAccuracy: cvAccuracy,
		CalibrationQueries:    queries,
		FamilyPrediction:      winningFamily,
		FamilyPredictionName:  familyNames[winningFamily],
		FamilyProbability:     familyProbabilities[winningFamily],
		FamilyProbabilities:   familyResults,
		Method:                "统一全局稳健数字指纹",
	}, nil
}
