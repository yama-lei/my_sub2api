// Package modeltrace identifies which LLM produced a set of "pick random
// integers" answers by comparing the number distribution against a bank of
// per-model fingerprints.
//
// This is a Go port of the ModelTrace scorer (static/fingerprint-core.js) and
// prompt generator (fingerprint.py generate_challenges) from
// https://github.com/xqy2006/ModelTrace, Copyright (c) 2026 xqy2006, released
// under the MIT License. The fingerprint bank (unified_bank.json) is copied
// verbatim from that project; see LICENSE.modeltrace for the full license text.
package modeltrace

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"sync"
)

const (
	valueMin           = 1
	valueMax           = 355
	dimension          = valueMax - valueMin + 1
	alpha              = 0.5
	minimumNumbers     = 80
	minimumCountFactor = 0.55
)

// ErrNoAcceptedOutput is returned by Analyze when no output contains enough
// parsable numbers (refusals or heavily truncated answers).
var ErrNoAcceptedOutput = errors.New("modeltrace: no usable output (refused or truncated answers are not counted)")

//go:embed unified_bank.json
var bankJSON []byte

type bankModel struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Family      string    `json:"family"`
	FamilyName  string    `json:"family_name"`
	Counts      []float64 `json:"counts"`
}

type hellingerArtifact struct {
	FeatureMean   []float64   `json:"feature_mean"`
	FeatureScale  []float64   `json:"feature_scale"`
	NuisanceBasis [][]float64 `json:"nuisance_basis"`
	Centroids     [][]float64 `json:"centroids"`
}

type orderedBlocksArtifact struct {
	Weight               float64       `json:"weight"`
	FeatureMean          []float64     `json:"feature_mean"`
	FeatureScale         []float64     `json:"feature_scale"`
	NuisanceBasis        [][]float64   `json:"nuisance_basis"`
	Centroids            [][]float64   `json:"centroids"`
	EnvironmentCentroids [][][]float64 `json:"environment_centroids"`
}

type calibrationEntry struct {
	Beta       float64 `json:"beta"`
	CVAccuracy float64 `json:"cv_accuracy"`
}

type bank struct {
	Models []bankModel `json:"models"`
	Robust struct {
		Hellinger     hellingerArtifact      `json:"hellinger"`
		OrderedBlocks *orderedBlocksArtifact `json:"ordered_blocks"`
	} `json:"robust"`
	Calibration map[string]calibrationEntry `json:"calibration"`
}

var (
	bankOnce   sync.Once
	loadedBank *bank
	bankErr    error
)

func getBank() (*bank, error) {
	bankOnce.Do(func() {
		var b bank
		if err := json.Unmarshal(bankJSON, &b); err != nil {
			bankErr = fmt.Errorf("modeltrace: parse embedded bank: %w", err)
			return
		}
		if len(b.Models) == 0 {
			bankErr = errors.New("modeltrace: embedded bank has no models")
			return
		}
		loadedBank = &b
	})
	return loadedBank, bankErr
}

// Output is one model answer to a challenge prompt.
type Output struct {
	Text          string
	ExpectedCount int
}

// ModelResult is the score of one bank model.
type ModelResult struct {
	Model                  string
	DisplayName            string
	Family                 string
	FamilyName             string
	Probability            float64
	ConditionalProbability float64 // probability within the model's family
	ProfileSimilarity      float64 // 1 - normalized Jensen-Shannon distance to the model's pooled counts
	Score                  float64
}

// Diagnostic describes whether an individual output was accepted.
type Diagnostic struct {
	Index          int
	ParsedNumbers  int
	MinimumNumbers int
	Accepted       bool
}

// Analysis is the aggregated verdict over all accepted outputs.
type Analysis struct {
	Prediction         string
	PredictionName     string
	Probability        float64
	UsedOutputs        int
	Results            []ModelResult // sorted by Probability, descending
	FamilyPrediction   string
	FamilyProbability  float64
	CalibrationQueries string
	CVAccuracy         float64
	Diagnostics        []Diagnostic
}

var (
	digitRun     = regexp.MustCompile(`[0-9]+`)
	letterInText = regexp.MustCompile(`\p{L}`)
)

// ParseNumbers extracts the longest run of integers in [1, 355] from text.
// Any Unicode letter between two numbers splits runs; out-of-range numbers are
// dropped without splitting. Ties keep the earliest run.
func ParseNumbers(text string) []int {
	var runs [][]int
	var current []int
	previousEnd := 0
	for _, loc := range digitRun.FindAllStringIndex(text, -1) {
		separator := text[previousEnd:loc[0]]
		value := digitValue(text[loc[0]:loc[1]])
		if len(current) > 0 && letterInText.MatchString(separator) {
			runs = append(runs, current)
			current = nil
		}
		if value >= valueMin && value <= valueMax {
			current = append(current, value)
		}
		previousEnd = loc[1]
	}
	if len(current) > 0 {
		runs = append(runs, current)
	}
	best := []int{}
	for _, run := range runs {
		if len(run) > len(best) {
			best = run
		}
	}
	return best
}

// digitValue parses an ASCII digit string, returning -1 for values that cannot
// fall in range (avoids overflow on very long runs).
func digitValue(digits string) int {
	i := 0
	for i < len(digits)-1 && digits[i] == '0' {
		i++
	}
	digits = digits[i:]
	if len(digits) > 4 {
		return -1
	}
	v, err := strconv.Atoi(digits)
	if err != nil {
		return -1
	}
	return v
}

// KnownModels returns the bank model IDs in bank order.
func KnownModels() []string {
	b, err := getBank()
	if err != nil {
		return nil
	}
	ids := make([]string, len(b.Models))
	for i, m := range b.Models {
		ids[i] = m.ID
	}
	return ids
}

// ProbabilityOf returns the probability assigned to model in a.
func ProbabilityOf(a *Analysis, model string) (float64, bool) {
	if a == nil {
		return 0, false
	}
	for _, r := range a.Results {
		if r.Model == model {
			return r.Probability, true
		}
	}
	return 0, false
}

// MinimumNumbersFor returns how many parsed numbers an output needs to be
// accepted, given the count requested in the prompt (0 = unknown).
func MinimumNumbersFor(expected int) int {
	if expected == 0 {
		return minimumNumbers
	}
	return int(math.Max(minimumNumbers, math.Ceil(float64(expected)*minimumCountFactor)))
}

type validOutput struct {
	counts []float64
	scores []float64
}

// Analyze scores outputs against the fingerprint bank. It returns
// ErrNoAcceptedOutput when no output has enough parsable numbers.
func Analyze(outputs []Output) (*Analysis, error) {
	b, err := getBank()
	if err != nil {
		return nil, err
	}
	diagnostics := make([]Diagnostic, 0, len(outputs))
	var valid []validOutput
	for index, item := range outputs {
		numbers := ParseNumbers(item.Text)
		minimum := MinimumNumbersFor(item.ExpectedCount)
		accepted := len(numbers) >= minimum
		diagnostics = append(diagnostics, Diagnostic{Index: index, ParsedNumbers: len(numbers), MinimumNumbers: minimum, Accepted: accepted})
		if accepted {
			valid = append(valid, validOutput{counts: countNumbers(numbers), scores: robustScoreNumbers(numbers, b)})
		}
	}
	if len(valid) == 0 {
		return nil, ErrNoAcceptedOutput
	}

	nModels := len(b.Models)
	combined := make([]float64, nModels)
	for i := range combined {
		column := make([]float64, len(valid))
		for j, v := range valid {
			column[j] = v.scores[i]
		}
		combined[i] = mean(column)
	}
	calibrationKey := strconv.Itoa(min(len(valid), 3))
	calibration, ok := b.Calibration[calibrationKey]
	if !ok {
		return nil, fmt.Errorf("modeltrace: bank missing calibration %q", calibrationKey)
	}
	scaled := make([]float64, nModels)
	for i, v := range combined {
		scaled[i] = calibration.Beta * v
	}
	probabilities := softmax(scaled)

	pooled := make([]float64, dimension)
	for i := range pooled {
		sum := 0.0
		for _, v := range valid {
			sum += v.counts[i]
		}
		pooled[i] = sum
	}

	familyOf := func(m bankModel) string {
		if m.Family == "" {
			return "models"
		}
		return m.Family
	}
	var familyOrder []string
	familyNames := map[string]string{}
	for _, m := range b.Models {
		f := familyOf(m)
		if _, seen := familyNames[f]; seen {
			continue
		}
		familyOrder = append(familyOrder, f)
		name := m.FamilyName
		if name == "" {
			name = f
		}
		familyNames[f] = name
	}

	results := make([]ModelResult, nModels)
	for i, m := range b.Models {
		f := familyOf(m)
		results[i] = ModelResult{
			Model:             m.ID,
			DisplayName:       m.DisplayName,
			Family:            f,
			FamilyName:        familyNames[f],
			Probability:       probabilities[i],
			ProfileSimilarity: jsSimilarity(pooled, m.Counts),
			Score:             combined[i],
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Probability > results[j].Probability })

	familyProbabilities := map[string]float64{}
	for _, f := range familyOrder {
		sum := 0.0
		for _, r := range results {
			if r.Family == f {
				sum += r.Probability
			}
		}
		familyProbabilities[f] = sum
	}
	for i := range results {
		results[i].ConditionalProbability = results[i].Probability / familyProbabilities[results[i].Family]
	}
	winning := familyOrder[0]
	for _, f := range familyOrder {
		if familyProbabilities[f] > familyProbabilities[winning] {
			winning = f
		}
	}

	return &Analysis{
		Prediction:         results[0].Model,
		PredictionName:     results[0].DisplayName,
		Probability:        results[0].Probability,
		UsedOutputs:        len(valid),
		Results:            results,
		FamilyPrediction:   winning,
		FamilyProbability:  familyProbabilities[winning],
		CalibrationQueries: calibrationKey,
		CVAccuracy:         calibration.CVAccuracy,
		Diagnostics:        diagnostics,
	}, nil
}

func countNumbers(numbers []int) []float64 {
	counts := make([]float64, dimension)
	for _, n := range numbers {
		counts[n-valueMin]++
	}
	return counts
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

func mean(values []float64) float64 {
	return sum(values) / float64(len(values))
}

func standardize(values []float64) []float64 {
	center := mean(values)
	squares := make([]float64, len(values))
	for i, v := range values {
		squares[i] = (v - center) * (v - center)
	}
	scale := math.Max(math.Sqrt(mean(squares)), 1e-12)
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = (v - center) / scale
	}
	return out
}

func dot(left, right []float64) float64 {
	value := 0.0
	for i := range left {
		value += left[i] * right[i]
	}
	return value
}

func normalized(values []float64) []float64 {
	scale := math.Max(math.Sqrt(dot(values, values)), 1e-12)
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = v / scale
	}
	return out
}

func subtractBasis(values []float64, basis [][]float64) []float64 {
	out := append([]float64(nil), values...)
	for _, vector := range basis {
		projection := dot(out, vector)
		for i := range out {
			out[i] -= projection * vector[i]
		}
	}
	return out
}

func standardizeAgainst(feature, featureMean, featureScale []float64) []float64 {
	out := make([]float64, len(feature))
	for i, v := range feature {
		out[i] = (v - featureMean[i]) / featureScale[i]
	}
	return out
}

func hellingerFeature(counts []float64) []float64 {
	total := sum(counts) + alpha*dimension
	out := make([]float64, len(counts))
	for i, v := range counts {
		out[i] = math.Sqrt((v + alpha) / total)
	}
	return out
}

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
		chunks = append(chunks, values[start:start+size])
		start += size
	}
	return chunks
}

func orderedBlockFeature(numbers []int) []float64 {
	pieces := make([]float64, 0, 4*16+10)
	for _, chunk := range splitIntoFour(numbers) {
		bins := make([]float64, 16)
		for i := range bins {
			bins[i] = 0.5
		}
		for _, value := range chunk {
			index := min(15, int(math.Floor((float64(value-1)/355)*16)))
			bins[index]++
		}
		total := sum(bins)
		for _, v := range bins {
			pieces = append(pieces, math.Sqrt(v/total))
		}
	}
	lastDigits := make([]float64, 10)
	for i := range lastDigits {
		lastDigits[i] = 0.5
	}
	for _, value := range numbers {
		lastDigits[value%10]++
	}
	lastTotal := sum(lastDigits)
	for _, v := range lastDigits {
		pieces = append(pieces, math.Sqrt(v/lastTotal))
	}
	return pieces
}

func robustScoreCounts(counts []float64, b *bank) []float64 {
	artifact := &b.Robust.Hellinger
	projected := standardizeAgainst(hellingerFeature(counts), artifact.FeatureMean, artifact.FeatureScale)
	projected = normalized(subtractBasis(projected, artifact.NuisanceBasis))
	scores := make([]float64, len(artifact.Centroids))
	for i, centroid := range artifact.Centroids {
		scores[i] = dot(projected, centroid)
	}
	return standardize(scores)
}

func orderedBlockScores(numbers []int, artifact *orderedBlocksArtifact) []float64 {
	standardizedFeature := standardizeAgainst(orderedBlockFeature(numbers), artifact.FeatureMean, artifact.FeatureScale)
	unit := normalized(standardizedFeature)
	nModels := len(artifact.Centroids)
	template := make([]float64, nModels)
	for m := 0; m < nModels; m++ {
		best := math.Inf(-1)
		for _, env := range artifact.EnvironmentCentroids {
			best = math.Max(best, dot(unit, env[m]))
		}
		template[m] = best
	}
	template = standardize(template)
	projected := normalized(subtractBasis(standardizedFeature, artifact.NuisanceBasis))
	nuisance := make([]float64, nModels)
	for i, centroid := range artifact.Centroids {
		nuisance[i] = dot(projected, centroid)
	}
	nuisance = standardize(nuisance)
	fused := make([]float64, nModels)
	for i := range fused {
		fused[i] = 0.5*template[i] + 0.5*nuisance[i]
	}
	return standardize(fused)
}

func robustScoreNumbers(numbers []int, b *bank) []float64 {
	marginal := robustScoreCounts(countNumbers(numbers), b)
	artifact := b.Robust.OrderedBlocks
	if artifact == nil || artifact.Weight == 0 {
		return marginal
	}
	ordered := orderedBlockScores(numbers, artifact)
	out := make([]float64, len(marginal))
	for i, v := range marginal {
		out[i] = (1-artifact.Weight)*v + artifact.Weight*ordered[i]
	}
	return out
}

func softmax(values []float64) []float64 {
	maximum := math.Inf(-1)
	for _, v := range values {
		maximum = math.Max(maximum, v)
	}
	weights := make([]float64, len(values))
	for i, v := range values {
		weights[i] = math.Exp(v - maximum)
	}
	total := sum(weights)
	for i := range weights {
		weights[i] /= total
	}
	return weights
}

func jsSimilarity(left, right []float64) float64 {
	leftTotal := sum(left)
	rightTotal := sum(right) + alpha*dimension
	p := make([]float64, len(left))
	q := make([]float64, len(left))
	midpoint := make([]float64, len(left))
	for i := range left {
		p[i] = left[i] / leftTotal
		q[i] = (right[i] + alpha) / rightTotal
		midpoint[i] = (p[i] + q[i]) / 2
	}
	divergence := func(values []float64) float64 {
		total := 0.0
		for i, v := range values {
			if v != 0 {
				total += v * math.Log(v/midpoint[i])
			}
		}
		return total
	}
	js := (divergence(p) + divergence(q)) / 2
	return 1 - math.Sqrt(js/math.Ln2)
}
