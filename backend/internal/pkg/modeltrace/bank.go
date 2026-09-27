// Package modeltrace 是 ModelTrace (https://github.com/xqy2006/ModelTrace)
// 主动模型归因算法的 Go 移植：通过「长整数第一反应序列」挑战提取模型数字指纹，
// 与内嵌的统一指纹库比对，判定实际响应的模型是否与声称的模型一致（是否掺水/套壳）。
//
// 移植自上游 fingerprint.py / static/fingerprint-core.js，语义保持一致：
//
//	fused = 0.75 × 去除环境方向后的 Hellinger 中心相似度 + 0.25 × 有序块数字序列特征
//	P(模型) = softmax(β × 全局模型得分)
package modeltrace

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

//go:embed data/unified_bank.json
var bankFS embed.FS

const (
	// ValueMin / ValueMax 是挑战整数的取值范围（含端点）。
	ValueMin = 1
	ValueMax = 355
	// Dimension 是数字直方图维度。
	Dimension = ValueMax - ValueMin + 1
	// Alpha 是 Hellinger / JS 平滑系数。
	Alpha = 0.5
	// OrderedBlockWeight 是有序块特征的融合权重。
	OrderedBlockWeight = 0.25
)

// FamilyGPT / FamilyClaude 是指纹库中的两个模型家族 ID。
const (
	FamilyGPT    = "gpt"
	FamilyClaude = "claude"
)

// ModelInfo 是指纹库中一个已收录模型的静态信息。
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Family      string `json:"family"`
	FamilyName  string `json:"family_name"`
}

// hellingerArtifact 保存边际 Hellinger 特征的稳健归因参数。
type hellingerArtifact struct {
	FeatureMean    []float64   `json:"feature_mean"`
	FeatureScale   []float64   `json:"feature_scale"`
	NuisanceBasis  [][]float64 `json:"nuisance_basis"`
	Centroids      [][]float64 `json:"centroids"`
}

// orderedBlocksArtifact 保存有序块特征的稳健归因参数。
type orderedBlocksArtifact struct {
	Weight              float64       `json:"weight"`
	FeatureMean         []float64     `json:"feature_mean"`
	FeatureScale        []float64     `json:"feature_scale"`
	NuisanceBasis       [][]float64   `json:"nuisance_basis"`
	Centroids           [][]float64   `json:"centroids"`
	// EnvironmentCentroids 形状为 [环境数][模型数][特征维]（12×16×74）。
	EnvironmentCentroids [][][]float64 `json:"environment_centroids"`
}

// robustArtifact 是 bank["robust"] 的结构。
type robustArtifact struct {
	ModelOrder   []string               `json:"model_order"`
	Hellinger    hellingerArtifact      `json:"hellinger"`
	OrderedBlocks *orderedBlocksArtifact `json:"ordered_blocks"`
}

// calibrationEntry 是每个查询数量的温度校准参数。
type calibrationEntry struct {
	Beta       float64 `json:"beta"`
	CVAccuracy float64 `json:"cv_accuracy"`
}

// bankModel 是指纹库中模型的完整条目（含直方图）。
type bankModel struct {
	ModelInfo
	Counts []int `json:"counts"`
}

// Bank 是统一指纹库。
type Bank struct {
	Schema      string                      `json:"schema"`
	BuiltAt     string                      `json:"built_at"`
	Models      []bankModel                 `json:"models"`
	Robust      robustArtifact              `json:"robust"`
	Calibration map[string]calibrationEntry `json:"calibration"`

	modelIndex map[string]int
}

var (
	bankOnce sync.Once
	bankErr  error
	loaded   *Bank
)

// LoadBank 从内嵌资源加载统一指纹库（进程内单例）。
func LoadBank() (*Bank, error) {
	bankOnce.Do(func() {
		raw, err := bankFS.ReadFile("data/unified_bank.json")
		if err != nil {
			bankErr = fmt.Errorf("modeltrace: 读取内嵌指纹库失败: %w", err)
			return
		}
		loaded, bankErr = ParseBank(raw)
	})
	return loaded, bankErr
}

// ParseBank 解析指纹库 JSON（供测试与外部注入使用）。
func ParseBank(raw []byte) (*Bank, error) {
	var bank Bank
	if err := json.Unmarshal(raw, &bank); err != nil {
		return nil, fmt.Errorf("modeltrace: 解析指纹库失败: %w", err)
	}
	bank.modelIndex = make(map[string]int, len(bank.Models))
	for i, model := range bank.Models {
		bank.modelIndex[model.ID] = i
	}
	return &bank, nil
}

// LoadBankFromFile 从文件加载指纹库（测试辅助）。
func LoadBankFromFile(path string) (*Bank, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseBank(raw)
}

// SupportedModels 返回指纹库收录的全部模型静态信息。
func (b *Bank) SupportedModels() []ModelInfo {
	out := make([]ModelInfo, 0, len(b.Models))
	for _, model := range b.Models {
		out = append(out, model.ModelInfo)
	}
	return out
}

// HasModel 判断模型是否在指纹库中。
func (b *Bank) HasModel(id string) bool {
	_, ok := b.modelIndex[id]
	return ok
}

// CalibrationBeta 返回 valid 份有效回答对应的校准温度 β。
func (b *Bank) CalibrationBeta(validCount int) (beta float64, cvAccuracy float64, err error) {
	key := validCount
	if key > 3 {
		key = 3
	}
	entry, ok := b.Calibration[fmt.Sprintf("%d", key)]
	if !ok {
		return 0, 0, fmt.Errorf("modeltrace: 指纹库缺少 %d 份回答的校准参数", key)
	}
	return entry.Beta, entry.CVAccuracy, nil
}
