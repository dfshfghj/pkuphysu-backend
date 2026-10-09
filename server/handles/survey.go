package handles

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	minSurveyQuestions       = 1
	maxSurveyQuestions       = 50
	maxSurveyBlocks          = 100
	maxSurveyMarkdownLength  = 10000
	minSurveyOptions         = 2
	maxSurveyOptions         = 50
	maxSurveyOptionLength    = 200
	defaultSurveyTextLength  = 200
	maxSurveyTextLength      = 2000
	maxSurveyScaleSpan       = 10
	surveySliderBucketCount  = 10
	surveyResultTextLimit    = 500
)

var surveyQuestionTypes = map[string]bool{
	model.SurveyQuestionSingle:     true,
	model.SurveyQuestionMultiple:   true,
	model.SurveyQuestionText:       true,
	model.SurveyQuestionRanking:    true,
	model.SurveyQuestionSlider:     true,
	model.SurveyQuestionProportion: true,
	model.SurveyQuestionScale:      true,
}

var surveyQuestionTypeLabels = map[string]string{
	model.SurveyQuestionSingle:     "单选",
	model.SurveyQuestionMultiple:   "多选",
	model.SurveyQuestionText:       "简答",
	model.SurveyQuestionRanking:    "排序",
	model.SurveyQuestionSlider:     "滑动条",
	model.SurveyQuestionProportion: "比重条",
	model.SurveyQuestionScale:      "量表",
}

type surveyBlockInput struct {
	Kind     string          `json:"kind"`
	Content  string          `json:"content"`
	Type     string          `json:"type"`
	Required bool            `json:"required"`
	Config   json.RawMessage `json:"config"`
}

type surveyInput struct {
	Status        string             `json:"status"`
	AllowMultiple bool               `json:"allow_multiple"`
	Blocks        []surveyBlockInput `json:"blocks"`
}

type surveyAnswerInput struct {
	BlockID uint            `json:"block_id"`
	Value   json.RawMessage `json:"value"`
}

type surveyResponseInput struct {
	Answers []surveyAnswerInput `json:"answers"`
}

type surveyChoiceConfig struct {
	Options []string `json:"options"`
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
}

type surveyTextConfig struct {
	Placeholder string `json:"placeholder,omitempty"`
	MaxLength   int    `json:"maxLength"`
}

type surveySliderConfig struct {
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Step     float64 `json:"step"`
	Default  float64 `json:"default"`
	Unit     string  `json:"unit,omitempty"`
	MinLabel string  `json:"minLabel,omitempty"`
	MaxLabel string  `json:"maxLabel,omitempty"`
}

type surveyProportionConfig struct {
	Options []string `json:"options"`
	Total   float64  `json:"total"`
}

type surveyScaleConfig struct {
	Min      int    `json:"min"`
	Max      int    `json:"max"`
	MinLabel string `json:"minLabel,omitempty"`
	MaxLabel string `json:"maxLabel,omitempty"`
}

func normalizeSurveyOptions(options []string) ([]string, error) {
	normalized := make([]string, 0, len(options))
	seen := make(map[string]bool, len(options))
	for _, option := range options {
		text := strings.TrimSpace(option)
		if text == "" {
			continue
		}
		if utf8.RuneCountInString(text) > maxSurveyOptionLength {
			return nil, fmt.Errorf("选项最多 %d 个字符", maxSurveyOptionLength)
		}
		if seen[text] {
			continue
		}
		seen[text] = true
		normalized = append(normalized, text)
	}

	if len(normalized) < minSurveyOptions {
		return nil, fmt.Errorf("至少需要 %d 个选项", minSurveyOptions)
	}
	if len(normalized) > maxSurveyOptions {
		return nil, fmt.Errorf("最多 %d 个选项", maxSurveyOptions)
	}
	return normalized, nil
}

func decodeSurveyConfig(raw json.RawMessage, out interface{}) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("题目配置格式不正确")
	}
	return nil
}

func marshalSurveyConfig(cfg interface{}) (string, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func intPtrOrNil(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func normalizeSurveyQuestionConfig(qType string, raw json.RawMessage) (string, error) {
	switch qType {
	case model.SurveyQuestionSingle, model.SurveyQuestionRanking:
		var cfg surveyChoiceConfig
		if err := decodeSurveyConfig(raw, &cfg); err != nil {
			return "", err
		}
		options, err := normalizeSurveyOptions(cfg.Options)
		if err != nil {
			return "", err
		}
		return marshalSurveyConfig(surveyChoiceConfig{Options: options})

	case model.SurveyQuestionMultiple:
		var cfg surveyChoiceConfig
		if err := decodeSurveyConfig(raw, &cfg); err != nil {
			return "", err
		}
		options, err := normalizeSurveyOptions(cfg.Options)
		if err != nil {
			return "", err
		}
		min, max := 0, 0
		if cfg.Min != nil {
			min = *cfg.Min
		}
		if cfg.Max != nil {
			max = *cfg.Max
		}
		if min < 0 {
			min = 0
		}
		if max < 0 {
			max = 0
		}
		if min > len(options) {
			return "", errors.New("最少选择数超过选项数")
		}
		if max > len(options) {
			max = len(options)
		}
		if max > 0 && min > max {
			return "", errors.New("最少选择数不能大于最多选择数")
		}
		return marshalSurveyConfig(surveyChoiceConfig{Options: options, Min: intPtrOrNil(min), Max: intPtrOrNil(max)})

	case model.SurveyQuestionText:
		var cfg surveyTextConfig
		if err := decodeSurveyConfig(raw, &cfg); err != nil {
			return "", err
		}
		maxLength := cfg.MaxLength
		if maxLength <= 0 {
			maxLength = defaultSurveyTextLength
		}
		if maxLength > maxSurveyTextLength {
			maxLength = maxSurveyTextLength
		}
		return marshalSurveyConfig(surveyTextConfig{Placeholder: strings.TrimSpace(cfg.Placeholder), MaxLength: maxLength})

	case model.SurveyQuestionSlider:
		var cfg surveySliderConfig
		if err := decodeSurveyConfig(raw, &cfg); err != nil {
			return "", err
		}
		if cfg.Max <= cfg.Min {
			return "", errors.New("滑动条最大值必须大于最小值")
		}
		if cfg.Step <= 0 {
			return "", errors.New("滑动条步长必须大于 0")
		}
		def := cfg.Default
		if def < cfg.Min {
			def = cfg.Min
		}
		if def > cfg.Max {
			def = cfg.Max
		}
		return marshalSurveyConfig(surveySliderConfig{
			Min: cfg.Min, Max: cfg.Max, Step: cfg.Step, Default: def,
			Unit: strings.TrimSpace(cfg.Unit), MinLabel: strings.TrimSpace(cfg.MinLabel), MaxLabel: strings.TrimSpace(cfg.MaxLabel),
		})

	case model.SurveyQuestionProportion:
		var cfg surveyProportionConfig
		if err := decodeSurveyConfig(raw, &cfg); err != nil {
			return "", err
		}
		options, err := normalizeSurveyOptions(cfg.Options)
		if err != nil {
			return "", err
		}
		total := cfg.Total
		if total <= 0 {
			total = 100
		}
		return marshalSurveyConfig(surveyProportionConfig{Options: options, Total: total})

	case model.SurveyQuestionScale:
		var cfg surveyScaleConfig
		if err := decodeSurveyConfig(raw, &cfg); err != nil {
			return "", err
		}
		if cfg.Max <= cfg.Min {
			return "", errors.New("量表最大值必须大于最小值")
		}
		if cfg.Max-cfg.Min > maxSurveyScaleSpan {
			return "", fmt.Errorf("量表档位不能超过 %d 档", maxSurveyScaleSpan+1)
		}
		return marshalSurveyConfig(surveyScaleConfig{
			Min: cfg.Min, Max: cfg.Max,
			MinLabel: strings.TrimSpace(cfg.MinLabel), MaxLabel: strings.TrimSpace(cfg.MaxLabel),
		})
	}
	return "", errors.New("不支持的题型")
}

func normalizeSurveyBlock(input surveyBlockInput, position int) (model.ForumSurveyBlock, error) {
	var block model.ForumSurveyBlock

	switch strings.TrimSpace(input.Kind) {
	case model.SurveyBlockMarkdown:
		content := strings.TrimSpace(input.Content)
		if content == "" {
			return block, errors.New("Markdown 内容不能为空")
		}
		if utf8.RuneCountInString(content) > maxSurveyMarkdownLength {
			return block, fmt.Errorf("Markdown 内容最多 %d 个字符", maxSurveyMarkdownLength)
		}
		return model.ForumSurveyBlock{
			Kind:     model.SurveyBlockMarkdown,
			Content:  content,
			Position: position,
			Config:   "{}",
		}, nil

	case model.SurveyBlockQuestion:
		qType := strings.TrimSpace(input.Type)
		if !surveyQuestionTypes[qType] {
			return block, errors.New("不支持的题型")
		}
		config, err := normalizeSurveyQuestionConfig(qType, input.Config)
		if err != nil {
			return block, err
		}
		return model.ForumSurveyBlock{
			Kind:     model.SurveyBlockQuestion,
			Type:     qType,
			Required: input.Required,
			Position: position,
			Config:   config,
		}, nil
	}
	return block, errors.New("不支持的区块类型")
}

// normalizeSurveyInput 清洗问卷入参：丢弃空白 Markdown 区块，校验每道题，产出可直接落库的问卷
func normalizeSurveyInput(input surveyInput) (*model.ForumSurvey, error) {
	if len(input.Blocks) > maxSurveyBlocks {
		return nil, fmt.Errorf("问卷最多 %d 个区块", maxSurveyBlocks)
	}

	blocks := make([]model.ForumSurveyBlock, 0, len(input.Blocks))
	questionCount := 0
	for _, item := range input.Blocks {
		if strings.TrimSpace(item.Kind) == model.SurveyBlockMarkdown && strings.TrimSpace(item.Content) == "" {
			continue
		}
		block, err := normalizeSurveyBlock(item, len(blocks))
		if err != nil {
			if strings.TrimSpace(item.Kind) == model.SurveyBlockQuestion {
				return nil, fmt.Errorf("第 %d 题：%w", questionCount+1, err)
			}
			return nil, fmt.Errorf("第 %d 个区块：%w", len(blocks)+1, err)
		}
		if block.Kind == model.SurveyBlockQuestion {
			questionCount++
		}
		blocks = append(blocks, block)
	}

	if questionCount < minSurveyQuestions {
		return nil, fmt.Errorf("问卷至少需要 %d 道题目", minSurveyQuestions)
	}
	if questionCount > maxSurveyQuestions {
		return nil, fmt.Errorf("问卷最多 %d 道题目", maxSurveyQuestions)
	}

	return &model.ForumSurvey{
		Status:        model.SurveyStatusOpen,
		AllowMultiple: input.AllowMultiple,
		Blocks:        blocks,
	}, nil
}

func isJSONEmpty(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" {
		return true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func parseSurveyChoiceConfig(config string) surveyChoiceConfig {
	var cfg surveyChoiceConfig
	json.Unmarshal([]byte(config), &cfg)
	return cfg
}

func parseSurveyTextConfig(config string) surveyTextConfig {
	var cfg surveyTextConfig
	json.Unmarshal([]byte(config), &cfg)
	return cfg
}

func parseSurveySliderConfig(config string) surveySliderConfig {
	var cfg surveySliderConfig
	json.Unmarshal([]byte(config), &cfg)
	return cfg
}

func parseSurveyProportionConfig(config string) surveyProportionConfig {
	var cfg surveyProportionConfig
	json.Unmarshal([]byte(config), &cfg)
	return cfg
}

func parseSurveyScaleConfig(config string) surveyScaleConfig {
	var cfg surveyScaleConfig
	json.Unmarshal([]byte(config), &cfg)
	return cfg
}

func validateSurveyAnswerValue(block *model.ForumSurveyBlock, raw json.RawMessage) (string, error) {
	switch block.Type {
	case model.SurveyQuestionSingle:
		options := parseSurveyChoiceConfig(block.Config).Options
		var index int
		if err := json.Unmarshal(raw, &index); err != nil {
			return "", errors.New("作答格式不正确")
		}
		if index < 0 || index >= len(options) {
			return "", errors.New("选项无效")
		}
		return strconv.Itoa(index), nil

	case model.SurveyQuestionScale:
		cfg := parseSurveyScaleConfig(block.Config)
		var value int
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("作答格式不正确")
		}
		if value < cfg.Min || value > cfg.Max {
			return "", errors.New("作答超出量表范围")
		}
		return strconv.Itoa(value), nil

	case model.SurveyQuestionSlider:
		cfg := parseSurveySliderConfig(block.Config)
		var value float64
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("作答格式不正确")
		}
		if value < cfg.Min || value > cfg.Max {
			return "", errors.New("作答超出滑动条范围")
		}
		return marshalSurveyValue(value)

	case model.SurveyQuestionText:
		cfg := parseSurveyTextConfig(block.Config)
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", errors.New("作答格式不正确")
		}
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) > cfg.MaxLength {
			return "", fmt.Errorf("作答超过 %d 个字符", cfg.MaxLength)
		}
		return marshalSurveyValue(text)

	case model.SurveyQuestionMultiple:
		options := parseSurveyChoiceConfig(block.Config).Options
		cfg := parseSurveyChoiceConfig(block.Config)
		var indices []int
		if err := json.Unmarshal(raw, &indices); err != nil {
			return "", errors.New("作答格式不正确")
		}
		seen := make(map[int]bool, len(indices))
		unique := make([]int, 0, len(indices))
		for _, index := range indices {
			if index < 0 || index >= len(options) {
				return "", errors.New("选项无效")
			}
			if seen[index] {
				continue
			}
			seen[index] = true
			unique = append(unique, index)
		}
		if len(unique) == 0 {
			return "", errors.New("请至少选择一项")
		}
		if cfg.Min != nil && len(unique) < *cfg.Min {
			return "", fmt.Errorf("至少选择 %d 项", *cfg.Min)
		}
		if cfg.Max != nil && len(unique) > *cfg.Max {
			return "", fmt.Errorf("最多选择 %d 项", *cfg.Max)
		}
		return marshalSurveyValue(unique)

	case model.SurveyQuestionRanking:
		options := parseSurveyChoiceConfig(block.Config).Options
		var order []int
		if err := json.Unmarshal(raw, &order); err != nil {
			return "", errors.New("作答格式不正确")
		}
		if len(order) != len(options) {
			return "", errors.New("需要为全部选项排序")
		}
		seen := make(map[int]bool, len(order))
		for _, index := range order {
			if index < 0 || index >= len(options) || seen[index] {
				return "", errors.New("排序无效")
			}
			seen[index] = true
		}
		return marshalSurveyValue(order)

	case model.SurveyQuestionProportion:
		cfg := parseSurveyProportionConfig(block.Config)
		var values []float64
		if err := json.Unmarshal(raw, &values); err != nil {
			return "", errors.New("作答格式不正确")
		}
		if len(values) != len(cfg.Options) {
			return "", errors.New("比重项数不正确")
		}
		sum := 0.0
		for _, value := range values {
			if value < 0 {
				return "", errors.New("比重不能为负")
			}
			sum += value
		}
		if math.Abs(sum-cfg.Total) > 0.01 {
			return "", fmt.Errorf("比重之和必须等于 %v", cfg.Total)
		}
		return marshalSurveyValue(values)
	}
	return "", errors.New("不支持的题型")
}

func marshalSurveyValue(value interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// validateSurveyAnswers 校验并规范化一份答卷：按题型逐题校验，必答缺失即报错
func validateSurveyAnswers(survey *model.ForumSurvey, inputs []surveyAnswerInput) ([]model.ForumSurveyAnswer, error) {
	provided := make(map[uint]json.RawMessage, len(inputs))
	for _, input := range inputs {
		if _, dup := provided[input.BlockID]; dup {
			return nil, errors.New("同一题目不能重复作答")
		}
		provided[input.BlockID] = input.Value
	}

	known := make(map[uint]bool, len(survey.Blocks))
	for i := range survey.Blocks {
		if survey.Blocks[i].Kind == model.SurveyBlockQuestion {
			known[survey.Blocks[i].ID] = true
		}
	}
	for id := range provided {
		if !known[id] {
			return nil, errors.New("作答包含不属于该问卷的题目")
		}
	}

	answers := make([]model.ForumSurveyAnswer, 0, len(survey.Blocks))
	ordinal := 0
	for i := range survey.Blocks {
		block := &survey.Blocks[i]
		if block.Kind != model.SurveyBlockQuestion {
			continue
		}
		ordinal++
		raw, ok := provided[block.ID]
		if !ok || isJSONEmpty(raw) {
			if block.Required {
				return nil, fmt.Errorf("第 %d 题为必答", ordinal)
			}
			continue
		}
		value, err := validateSurveyAnswerValue(block, raw)
		if err != nil {
			return nil, fmt.Errorf("第 %d 题：%w", ordinal, err)
		}
		answers = append(answers, model.ForumSurveyAnswer{BlockID: block.ID, Value: value})
	}
	return answers, nil
}

func rawJSONOrNull(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return json.RawMessage("null")
	}
	return json.RawMessage(trimmed)
}

func countSurveyQuestions(blocks []model.ForumSurveyBlock) int {
	count := 0
	for i := range blocks {
		if blocks[i].Kind == model.SurveyBlockQuestion {
			count++
		}
	}
	return count
}

// buildSurveyMetaPayload 构建问卷摘要；答卷数只对作者/管理员下发
func buildSurveyMetaPayload(survey *model.ForumSurvey, responseCount int64, viewerID uint, isAdmin bool) map[string]interface{} {
	isOwner := survey.UserID == viewerID
	canViewResults := isOwner || isAdmin

	payload := map[string]interface{}{
		"id":               survey.ID,
		"question_count":   countSurveyQuestions(survey.Blocks),
		"status":           survey.Status,
		"allow_multiple":   survey.AllowMultiple,
		"is_owner":         isOwner,
		"can_view_results": canViewResults,
	}
	if canViewResults {
		payload["response_count"] = responseCount
	}
	return payload
}

// buildSurveyPayload 构建问卷详情（含区块），供填写页使用
func buildSurveyPayload(survey *model.ForumSurvey, viewerID uint, isAdmin bool, answered bool, responseCount int64) map[string]interface{} {
	payload := buildSurveyMetaPayload(survey, responseCount, viewerID, isAdmin)
	payload["answered"] = answered
	payload["can_edit"] = survey.UserID == viewerID && responseCount == 0

	blocks := make([]map[string]interface{}, 0, len(survey.Blocks))
	for _, block := range survey.Blocks {
		if block.Kind == model.SurveyBlockQuestion {
			blocks = append(blocks, map[string]interface{}{
				"id":       block.ID,
				"kind":     block.Kind,
				"type":     block.Type,
				"required": block.Required,
				"position": block.Position,
				"config":   rawJSONOrNull(block.Config),
			})
			continue
		}
		blocks = append(blocks, map[string]interface{}{
			"id":       block.ID,
			"kind":     block.Kind,
			"content":  block.Content,
			"position": block.Position,
		})
	}
	payload["blocks"] = blocks
	return payload
}

type surveyResponseView struct {
	Username string
	Answers  map[uint]json.RawMessage
}

func percentOf(count, total int) int {
	if total <= 0 {
		return 0
	}
	return int(math.Round(float64(count) / float64(total) * 100))
}

func optionResults(options []string, counts []int, total int) []map[string]interface{} {
	results := make([]map[string]interface{}, len(options))
	for i, option := range options {
		results[i] = map[string]interface{}{
			"text":    option,
			"count":   counts[i],
			"percent": percentOf(counts[i], total),
		}
	}
	return results
}

func buildSurveyBlockResult(block *model.ForumSurveyBlock, views []surveyResponseView) map[string]interface{} {
	base := map[string]interface{}{
		"id":       block.ID,
		"kind":     model.SurveyBlockQuestion,
		"type":     block.Type,
		"required": block.Required,
		"config":   rawJSONOrNull(block.Config),
	}

	switch block.Type {
	case model.SurveyQuestionSingle:
		options := parseSurveyChoiceConfig(block.Config).Options
		counts := make([]int, len(options))
		answered := 0
		for _, view := range views {
			raw, ok := view.Answers[block.ID]
			if !ok || isJSONEmpty(raw) {
				continue
			}
			var index int
			if json.Unmarshal(raw, &index) == nil && index >= 0 && index < len(options) {
				counts[index]++
				answered++
			}
		}
		base["answered"] = answered
		base["options"] = optionResults(options, counts, answered)

	case model.SurveyQuestionMultiple:
		options := parseSurveyChoiceConfig(block.Config).Options
		counts := make([]int, len(options))
		answered := 0
		for _, view := range views {
			raw, ok := view.Answers[block.ID]
			if !ok || isJSONEmpty(raw) {
				continue
			}
			var indices []int
			if json.Unmarshal(raw, &indices) != nil {
				continue
			}
			answered++
			for _, index := range indices {
				if index >= 0 && index < len(options) {
					counts[index]++
				}
			}
		}
		base["answered"] = answered
		base["options"] = optionResults(options, counts, answered)

	case model.SurveyQuestionScale:
		cfg := parseSurveyScaleConfig(block.Config)
		values := make([]map[string]interface{}, 0)
		sum, answered := 0.0, 0
		for value := cfg.Min; value <= cfg.Max; value++ {
			count := 0
			for _, view := range views {
				raw, ok := view.Answers[block.ID]
				if !ok || isJSONEmpty(raw) {
					continue
				}
				var got int
				if json.Unmarshal(raw, &got) == nil && got == value {
					count++
				}
			}
			sum += float64(value * count)
			answered += count
			values = append(values, map[string]interface{}{
				"value":   value,
				"count":   count,
				"percent": percentOf(count, len(views)),
			})
		}
		average := 0.0
		if answered > 0 {
			average = sum / float64(answered)
		}
		base["answered"] = answered
		base["min"] = cfg.Min
		base["max"] = cfg.Max
		base["min_label"] = cfg.MinLabel
		base["max_label"] = cfg.MaxLabel
		base["average"] = math.Round(average*100) / 100
		base["values"] = values

	case model.SurveyQuestionSlider:
		cfg := parseSurveySliderConfig(block.Config)
		numbers := make([]float64, 0, len(views))
		for _, view := range views {
			raw, ok := view.Answers[block.ID]
			if !ok || isJSONEmpty(raw) {
				continue
			}
			var value float64
			if json.Unmarshal(raw, &value) == nil {
				numbers = append(numbers, value)
			}
		}
		average, minValue, maxValue := 0.0, 0.0, 0.0
		if len(numbers) > 0 {
			minValue, maxValue = numbers[0], numbers[0]
			sum := 0.0
			for _, value := range numbers {
				sum += value
				if value < minValue {
					minValue = value
				}
				if value > maxValue {
					maxValue = value
				}
			}
			average = sum / float64(len(numbers))
		}
		width := (cfg.Max - cfg.Min) / float64(surveySliderBucketCount)
		buckets := make([]map[string]interface{}, 0, surveySliderBucketCount)
		for i := 0; i < surveySliderBucketCount; i++ {
			from := cfg.Min + width*float64(i)
			to := from + width
			count := 0
			for _, value := range numbers {
				if (value >= from && value < to) || (i == surveySliderBucketCount-1 && value >= from && value <= cfg.Max) {
					count++
				}
			}
			buckets = append(buckets, map[string]interface{}{
				"from":  math.Round(from*100) / 100,
				"to":    math.Round(to*100) / 100,
				"count": count,
			})
		}
		base["answered"] = len(numbers)
		base["min"] = cfg.Min
		base["max"] = cfg.Max
		base["unit"] = cfg.Unit
		base["average"] = math.Round(average*100) / 100
		base["min_value"] = minValue
		base["max_value"] = maxValue
		base["buckets"] = buckets

	case model.SurveyQuestionText:
		texts := make([]map[string]interface{}, 0)
		answered := 0
		for _, view := range views {
			raw, ok := view.Answers[block.ID]
			if !ok || isJSONEmpty(raw) {
				continue
			}
			var text string
			if json.Unmarshal(raw, &text) != nil {
				continue
			}
			answered++
			if len(texts) < surveyResultTextLimit {
				texts = append(texts, map[string]interface{}{
					"username": view.Username,
					"text":     text,
				})
			}
		}
		base["answered"] = answered
		base["texts"] = texts

	case model.SurveyQuestionRanking:
		options := parseSurveyChoiceConfig(block.Config).Options
		sums := make([]float64, len(options))
		counts := make([]int, len(options))
		answered := 0
		for _, view := range views {
			raw, ok := view.Answers[block.ID]
			if !ok || isJSONEmpty(raw) {
				continue
			}
			var order []int
			if json.Unmarshal(raw, &order) != nil || len(order) != len(options) {
				continue
			}
			answered++
			for rank, index := range order {
				if index >= 0 && index < len(options) {
					sums[index] += float64(rank + 1)
					counts[index]++
				}
			}
		}
		results := make([]map[string]interface{}, len(options))
		for i, option := range options {
			average := 0.0
			if counts[i] > 0 {
				average = sums[i] / float64(counts[i])
			}
			results[i] = map[string]interface{}{
				"text":         option,
				"average_rank": math.Round(average*100) / 100,
			}
		}
		base["answered"] = answered
		base["options"] = results

	case model.SurveyQuestionProportion:
		cfg := parseSurveyProportionConfig(block.Config)
		sums := make([]float64, len(cfg.Options))
		answered := 0
		for _, view := range views {
			raw, ok := view.Answers[block.ID]
			if !ok || isJSONEmpty(raw) {
				continue
			}
			var values []float64
			if json.Unmarshal(raw, &values) != nil || len(values) != len(cfg.Options) {
				continue
			}
			answered++
			for i, value := range values {
				sums[i] += value
			}
		}
		results := make([]map[string]interface{}, len(cfg.Options))
		for i, option := range cfg.Options {
			average := 0.0
			if answered > 0 {
				average = sums[i] / float64(answered)
			}
			results[i] = map[string]interface{}{
				"text":    option,
				"average": math.Round(average*100) / 100,
			}
		}
		base["answered"] = answered
		base["total"] = cfg.Total
		base["options"] = results
	}

	return base
}

// buildSurveyResults 聚合问卷统计结果，供数据后台使用
func buildSurveyResults(survey *model.ForumSurvey, responses []model.ForumSurveyResponse) map[string]interface{} {
	views := make([]surveyResponseView, 0, len(responses))
	for _, response := range responses {
		answers := make(map[uint]json.RawMessage, len(response.Answers))
		for _, answer := range response.Answers {
			answers[answer.BlockID] = json.RawMessage(answer.Value)
		}
		username := ""
		if response.User != nil {
			username = response.User.Username
		}
		views = append(views, surveyResponseView{Username: username, Answers: answers})
	}

	blocks := make([]map[string]interface{}, 0, len(survey.Blocks))
	for i := range survey.Blocks {
		block := &survey.Blocks[i]
		if block.Kind == model.SurveyBlockQuestion {
			blocks = append(blocks, buildSurveyBlockResult(block, views))
			continue
		}
		blocks = append(blocks, map[string]interface{}{
			"id":       block.ID,
			"kind":     block.Kind,
			"content":  block.Content,
			"position": block.Position,
		})
	}

	return map[string]interface{}{
		"id":             survey.ID,
		"status":         survey.Status,
		"allow_multiple": survey.AllowMultiple,
		"response_count": len(responses),
		"blocks":         blocks,
	}
}

func formatSurveyAnswerCSV(block *model.ForumSurveyBlock, raw json.RawMessage) string {
	if isJSONEmpty(raw) {
		return ""
	}

	switch block.Type {
	case model.SurveyQuestionSingle:
		options := parseSurveyChoiceConfig(block.Config).Options
		var index int
		if json.Unmarshal(raw, &index) == nil && index >= 0 && index < len(options) {
			return options[index]
		}
	case model.SurveyQuestionMultiple:
		options := parseSurveyChoiceConfig(block.Config).Options
		var indices []int
		if json.Unmarshal(raw, &indices) == nil {
			texts := make([]string, 0, len(indices))
			for _, index := range indices {
				if index >= 0 && index < len(options) {
					texts = append(texts, options[index])
				}
			}
			return strings.Join(texts, ";")
		}
	case model.SurveyQuestionRanking:
		options := parseSurveyChoiceConfig(block.Config).Options
		var order []int
		if json.Unmarshal(raw, &order) == nil {
			texts := make([]string, 0, len(order))
			for _, index := range order {
				if index >= 0 && index < len(options) {
					texts = append(texts, options[index])
				}
			}
			return strings.Join(texts, ">")
		}
	case model.SurveyQuestionProportion:
		cfg := parseSurveyProportionConfig(block.Config)
		var values []float64
		if json.Unmarshal(raw, &values) == nil && len(values) == len(cfg.Options) {
			parts := make([]string, 0, len(values))
			for i, value := range values {
				parts = append(parts, fmt.Sprintf("%s:%v", cfg.Options[i], value))
			}
			return strings.Join(parts, ";")
		}
	case model.SurveyQuestionText:
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return text
		}
	default:
		var value float64
		if json.Unmarshal(raw, &value) == nil {
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return strings.Trim(string(raw), "\"")
}

// buildSurveyCSV 将问卷的全部答卷导出为 CSV
func buildSurveyCSV(survey *model.ForumSurvey, responses []model.ForumSurveyResponse) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(&buffer)
	header := []string{"答卷ID", "答题人", "提交时间"}
	ordinal := 0
	for i := range survey.Blocks {
		if survey.Blocks[i].Kind != model.SurveyBlockQuestion {
			continue
		}
		ordinal++
		label := surveyQuestionTypeLabels[survey.Blocks[i].Type]
		header = append(header, fmt.Sprintf("第 %d 题（%s）", ordinal, label))
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, response := range responses {
		answers := make(map[uint]json.RawMessage, len(response.Answers))
		for _, answer := range response.Answers {
			answers[answer.BlockID] = json.RawMessage(answer.Value)
		}
		username := ""
		if response.User != nil {
			username = response.User.Username
		}
		row := []string{
			strconv.FormatUint(uint64(response.ID), 10),
			username,
			response.CreatedAt.Format(time.RFC3339),
		}
		for i := range survey.Blocks {
			block := &survey.Blocks[i]
			if block.Kind != model.SurveyBlockQuestion {
				continue
			}
			row = append(row, formatSurveyAnswerCSV(block, answers[block.ID]))
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// attachSurveys 把问卷摘要按帖子顺序注入响应，无问卷的帖子置为 null
func attachSurveys(postData []map[string]interface{}, posts []model.ForumPost, viewer *model.User) {
	if len(postData) == 0 || len(posts) == 0 {
		return
	}

	postIDs := make([]uint, len(posts))
	for i, post := range posts {
		postIDs[i] = post.ID
	}

	surveys, err := db.GetForumSurveysByPostIDs(postIDs)
	if err != nil {
		logrus.WithError(err).Warn("failed to load forum surveys")
		return
	}

	surveyIDs := make([]uint, 0, len(surveys))
	for _, survey := range surveys {
		surveyIDs = append(surveyIDs, survey.ID)
	}
	counts, err := db.GetSurveyResponseCounts(surveyIDs)
	if err != nil {
		logrus.WithError(err).Warn("failed to count survey responses")
		counts = map[uint]int64{}
	}

	for i := range postData {
		if i >= len(posts) {
			break
		}
		survey, ok := surveys[posts[i].ID]
		if !ok {
			postData[i]["survey"] = nil
			continue
		}
		postData[i]["survey"] = buildSurveyMetaPayload(survey, counts[survey.ID], viewer.ID, viewer.IsAdmin())
	}
}

func loadSurveyForViewer(c *gin.Context) (*model.ForumSurvey, *model.User, *model.ForumPost, bool) {
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return nil, nil, nil, false
	}

	survey, err := db.GetForumSurveyByID(uint(sid))
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return nil, nil, nil, false
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)
	post, err := db.GetForumPostByID(int(survey.PostID))
	if err != nil || !canViewForumContent(post.Status, post.UserID, currentUser.ID) {
		utils.RespondError(c, 404, "NotFound", nil)
		return nil, nil, nil, false
	}
	return survey, currentUser, post, true
}

// GetSurvey 获取问卷详情（含区块），供填写页使用
func GetSurvey(c *gin.Context) {
	survey, currentUser, post, ok := loadSurveyForViewer(c)
	if !ok {
		return
	}

	responseCount, err := db.GetSurveyResponseCount(survey.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}
	answered, err := db.HasUserSurveyResponse(survey.ID, currentUser.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	payload := buildSurveyPayload(survey, currentUser.ID, currentUser.IsAdmin(), answered, responseCount)
	payload["post_id"] = post.ID

	utils.RespondSuccess(c, payload)
}

// SubmitSurveyResponse 提交一份问卷答卷
func SubmitSurveyResponse(c *gin.Context) {
	survey, currentUser, _, ok := loadSurveyForViewer(c)
	if !ok {
		return
	}

	if survey.Status != model.SurveyStatusOpen {
		utils.RespondError(c, 403, "SurveyClosed", errors.New("问卷已关闭"))
		return
	}

	var req surveyResponseInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	answers, err := validateSurveyAnswers(survey, req.Answers)
	if err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	if err := db.CreateSurveyResponse(survey, currentUser.ID, answers); err != nil {
		if errors.Is(err, db.ErrAlreadySurveyed) {
			utils.RespondError(c, 409, "AlreadySurveyed", errors.New("您已经提交过该问卷"))
			return
		}
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "提交成功"})
}

// GetSurveyResults 获取问卷统计数据，仅作者与管理员可访问
func GetSurveyResults(c *gin.Context) {
	survey, currentUser, post, ok := loadSurveyForViewer(c)
	if !ok {
		return
	}
	if survey.UserID != currentUser.ID && !currentUser.IsAdmin() {
		utils.RespondError(c, 403, "Forbidden", errors.New("无权查看该问卷数据"))
		return
	}

	responses, err := db.GetSurveyResponses(survey.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	results := buildSurveyResults(survey, responses)
	results["post_id"] = post.ID

	utils.RespondSuccess(c, results)
}

// ExportSurveyResults 将问卷答卷导出为 CSV 文件，仅作者与管理员可访问
func ExportSurveyResults(c *gin.Context) {
	survey, currentUser, _, ok := loadSurveyForViewer(c)
	if !ok {
		return
	}
	if survey.UserID != currentUser.ID && !currentUser.IsAdmin() {
		utils.RespondError(c, 403, "Forbidden", errors.New("无权导出该问卷数据"))
		return
	}

	responses, err := db.GetSurveyResponses(survey.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	data, err := buildSurveyCSV(survey, responses)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"survey-%d.csv\"", survey.ID))
	c.Data(200, "text/csv; charset=utf-8", data)
}

// UpdateSurvey 作者更新问卷；已有答卷时仅可修改开关与限答设置
func UpdateSurvey(c *gin.Context) {
	survey, currentUser, _, ok := loadSurveyForViewer(c)
	if !ok {
		return
	}
	if survey.UserID != currentUser.ID {
		utils.RespondError(c, 403, "Forbidden", errors.New("无权修改该问卷"))
		return
	}

	var req surveyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	normalized, err := normalizeSurveyInput(req)
	if err != nil {
		utils.RespondError(c, 400, "InvalidSurvey", err)
		return
	}

	status := survey.Status
	if req.Status != "" {
		if req.Status != model.SurveyStatusOpen && req.Status != model.SurveyStatusClosed {
			utils.RespondError(c, 400, "InvalidStatus", errors.New("问卷状态无效"))
			return
		}
		status = req.Status
	}

	if err := db.UpdateSurveyMeta(survey.ID, status, normalized.AllowMultiple); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	responseCount, err := db.GetSurveyResponseCount(survey.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	locked := responseCount > 0
	if !locked {
		if err := db.ReplaceSurveyBlocks(survey.ID, normalized.Blocks); err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
	}

	message := "问卷已更新"
	if locked {
		message = "已有答卷，题目结构未修改"
	}

	utils.RespondSuccess(c, gin.H{"message": message, "locked": locked})
}
