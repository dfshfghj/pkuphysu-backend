package handles

import (
	"encoding/json"
	"strings"
	"testing"

	"pkuphysu-backend/internal/model"
)

func mustJSON(value interface{}) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func markdownBlock(content string) surveyBlockInput {
	return surveyBlockInput{Kind: model.SurveyBlockMarkdown, Content: content}
}

func questionBlock(qType string, required bool, config map[string]interface{}) surveyBlockInput {
	return surveyBlockInput{
		Kind:     model.SurveyBlockQuestion,
		Type:     qType,
		Required: required,
		Config:   mustJSON(config),
	}
}

func baseSurveyInput() surveyInput {
	return surveyInput{
		Blocks: []surveyBlockInput{
			markdownBlock("  # 校园问卷  "),
			questionBlock(model.SurveyQuestionSingle, true, map[string]interface{}{"options": []string{"A", "B"}}),
		},
	}
}

func TestNormalizeSurveyInputValid(t *testing.T) {
	survey, err := normalizeSurveyInput(baseSurveyInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if survey.Status != model.SurveyStatusOpen {
		t.Fatalf("unexpected survey: %#v", survey)
	}
	if len(survey.Blocks) != 2 {
		t.Fatalf("unexpected blocks: %#v", survey.Blocks)
	}

	md := survey.Blocks[0]
	if md.Kind != model.SurveyBlockMarkdown || md.Content != "# 校园问卷" || md.Position != 0 {
		t.Fatalf("unexpected markdown block: %#v", md)
	}

	q := survey.Blocks[1]
	if q.Kind != model.SurveyBlockQuestion || q.Type != model.SurveyQuestionSingle || q.Position != 1 {
		t.Fatalf("unexpected question block: %#v", q)
	}
	var cfg surveyChoiceConfig
	if err := json.Unmarshal([]byte(q.Config), &cfg); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	if len(cfg.Options) != 2 {
		t.Fatalf("unexpected options: %#v", cfg.Options)
	}
}

func TestNormalizeSurveyInputDropsBlankMarkdown(t *testing.T) {
	input := surveyInput{
		Blocks: []surveyBlockInput{
			markdownBlock("   "),
			questionBlock(model.SurveyQuestionSingle, false, map[string]interface{}{"options": []string{"A", "B"}}),
			markdownBlock("说明"),
		},
	}
	survey, err := normalizeSurveyInput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(survey.Blocks) != 2 {
		t.Fatalf("expected the blank markdown block to be dropped, got %#v", survey.Blocks)
	}
	if survey.Blocks[0].Kind != model.SurveyBlockQuestion || survey.Blocks[0].Position != 0 {
		t.Fatalf("positions not recomputed: %#v", survey.Blocks[0])
	}
	if survey.Blocks[1].Kind != model.SurveyBlockMarkdown || survey.Blocks[1].Position != 1 {
		t.Fatalf("unexpected second block: %#v", survey.Blocks[1])
	}
}

func TestNormalizeSurveyInputRejectsInvalid(t *testing.T) {
	noBlocks := surveyInput{}
	if _, err := normalizeSurveyInput(noBlocks); err == nil {
		t.Fatal("expected error for no blocks")
	}

	onlyMarkdown := surveyInput{Blocks: []surveyBlockInput{markdownBlock("只有文字")}}
	if _, err := normalizeSurveyInput(onlyMarkdown); err == nil {
		t.Fatal("expected error when there is no question block")
	}

	badKind := baseSurveyInput()
	badKind.Blocks[0] = surveyBlockInput{Kind: "video", Content: "x"}
	if _, err := normalizeSurveyInput(badKind); err == nil {
		t.Fatal("expected error for an unsupported block kind")
	}

	badType := baseSurveyInput()
	badType.Blocks[1].Type = "unknown"
	if _, err := normalizeSurveyInput(badType); err == nil {
		t.Fatal("expected error for an unsupported question type")
	}

	fewOptions := baseSurveyInput()
	fewOptions.Blocks[1].Config = mustJSON(map[string]interface{}{"options": []string{"A"}})
	if _, err := normalizeSurveyInput(fewOptions); err == nil {
		t.Fatal("expected error for too few options")
	}

	badSlider := baseSurveyInput()
	badSlider.Blocks[1] = questionBlock(model.SurveyQuestionSlider, false, map[string]interface{}{"min": 10, "max": 10, "step": 1})
	if _, err := normalizeSurveyInput(badSlider); err == nil {
		t.Fatal("expected error when slider max <= min")
	}

	badScale := baseSurveyInput()
	badScale.Blocks[1] = questionBlock(model.SurveyQuestionScale, false, map[string]interface{}{"min": 1, "max": 99})
	if _, err := normalizeSurveyInput(badScale); err == nil {
		t.Fatal("expected error when scale span is too large")
	}
}

func TestNormalizeSurveyInputNormalizesConfig(t *testing.T) {
	input := surveyInput{
		Blocks: []surveyBlockInput{
			questionBlock(model.SurveyQuestionMultiple, false, map[string]interface{}{
				"options": []string{" A ", "A", "", "B"},
				"min":     1,
				"max":     5,
			}),
			questionBlock(model.SurveyQuestionProportion, false, map[string]interface{}{"options": []string{"A", "B"}}),
		},
	}

	survey, err := normalizeSurveyInput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var multiple surveyChoiceConfig
	json.Unmarshal([]byte(survey.Blocks[0].Config), &multiple)
	if len(multiple.Options) != 2 || multiple.Options[0] != "A" || multiple.Options[1] != "B" {
		t.Fatalf("options not deduped/trimmed: %#v", multiple.Options)
	}
	if multiple.Min == nil || *multiple.Min != 1 || multiple.Max == nil || *multiple.Max != 2 {
		t.Fatalf("unexpected min/max: %#v", multiple)
	}

	var proportion surveyProportionConfig
	json.Unmarshal([]byte(survey.Blocks[1].Config), &proportion)
	if proportion.Total != 100 {
		t.Fatalf("expected proportion total to default to 100, got %v", proportion.Total)
	}
}

// allTypeSurvey 构造一个含 Markdown 区块与全部 7 种题型的问卷，并为每个区块分配 ID（index+1）。
func allTypeSurvey(t *testing.T) *model.ForumSurvey {
	t.Helper()
	input := surveyInput{
		Blocks: []surveyBlockInput{
			markdownBlock("题目说明"),
			questionBlock(model.SurveyQuestionSingle, true, map[string]interface{}{"options": []string{"A", "B"}}),
			questionBlock(model.SurveyQuestionMultiple, false, map[string]interface{}{"options": []string{"A", "B", "C"}, "min": 1, "max": 2}),
			questionBlock(model.SurveyQuestionText, false, map[string]interface{}{"maxLength": 5}),
			questionBlock(model.SurveyQuestionRanking, false, map[string]interface{}{"options": []string{"A", "B", "C"}}),
			questionBlock(model.SurveyQuestionProportion, false, map[string]interface{}{"options": []string{"A", "B"}, "total": 100}),
			questionBlock(model.SurveyQuestionScale, false, map[string]interface{}{"min": 1, "max": 5}),
			questionBlock(model.SurveyQuestionSlider, false, map[string]interface{}{"min": 0, "max": 10, "step": 1}),
		},
	}
	survey, err := normalizeSurveyInput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range survey.Blocks {
		survey.Blocks[i].ID = uint(i + 1)
	}
	return survey
}

func TestValidateSurveyAnswersAcceptsValid(t *testing.T) {
	survey := allTypeSurvey(t)
	answers := []surveyAnswerInput{
		{BlockID: 2, Value: mustJSON(1)},
		{BlockID: 3, Value: mustJSON([]int{0, 2})},
		{BlockID: 4, Value: mustJSON("hello")},
		{BlockID: 5, Value: mustJSON([]int{2, 0, 1})},
		{BlockID: 6, Value: mustJSON([]float64{40, 60})},
		{BlockID: 7, Value: mustJSON(4)},
		{BlockID: 8, Value: mustJSON(7.5)},
	}

	got, err := validateSurveyAnswers(survey, answers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("expected 7 answers, got %d", len(got))
	}
}

func TestValidateSurveyAnswersRejectsInvalid(t *testing.T) {
	survey := allTypeSurvey(t)

	cases := map[string][]surveyAnswerInput{
		"required missing": {
			{BlockID: 3, Value: mustJSON([]int{0})},
		},
		"single out of range": {
			{BlockID: 2, Value: mustJSON(5)},
		},
		"multiple over max": {
			{BlockID: 2, Value: mustJSON(0)},
			{BlockID: 3, Value: mustJSON([]int{0, 1, 2})},
		},
		"ranking not permutation": {
			{BlockID: 2, Value: mustJSON(0)},
			{BlockID: 5, Value: mustJSON([]int{0, 1})},
		},
		"proportion sum mismatch": {
			{BlockID: 2, Value: mustJSON(0)},
			{BlockID: 6, Value: mustJSON([]float64{30, 60})},
		},
		"text too long": {
			{BlockID: 2, Value: mustJSON(0)},
			{BlockID: 4, Value: mustJSON("toolong")},
		},
		"markdown block answered": {
			{BlockID: 2, Value: mustJSON(0)},
			{BlockID: 1, Value: mustJSON("x")},
		},
		"unknown question": {
			{BlockID: 2, Value: mustJSON(0)},
			{BlockID: 99, Value: mustJSON(1)},
		},
		"duplicate question": {
			{BlockID: 2, Value: mustJSON(0)},
			{BlockID: 2, Value: mustJSON(1)},
		},
	}

	for name, answers := range cases {
		if _, err := validateSurveyAnswers(survey, answers); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestValidateSurveyAnswersAllowsOptionalEmpty(t *testing.T) {
	survey := allTypeSurvey(t)
	got, err := validateSurveyAnswers(survey, []surveyAnswerInput{
		{BlockID: 2, Value: mustJSON(0)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the required answer, got %d", len(got))
	}
}

func surveyForResults(t *testing.T) *model.ForumSurvey {
	t.Helper()
	input := surveyInput{
		Blocks: []surveyBlockInput{
			markdownBlock("统计页说明"),
			questionBlock(model.SurveyQuestionSingle, false, map[string]interface{}{"options": []string{"A", "B"}}),
			questionBlock(model.SurveyQuestionScale, false, map[string]interface{}{"min": 1, "max": 5}),
			questionBlock(model.SurveyQuestionRanking, false, map[string]interface{}{"options": []string{"A", "B"}}),
			questionBlock(model.SurveyQuestionText, false, map[string]interface{}{"maxLength": 100}),
		},
	}
	survey, err := normalizeSurveyInput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range survey.Blocks {
		survey.Blocks[i].ID = uint(i + 1)
	}
	return survey
}

func responseWith(username string, answers map[uint]interface{}) model.ForumSurveyResponse {
	items := make([]model.ForumSurveyAnswer, 0, len(answers))
	for blockID, value := range answers {
		items = append(items, model.ForumSurveyAnswer{BlockID: blockID, Value: string(mustJSON(value))})
	}
	return model.ForumSurveyResponse{
		User:    &model.User{Username: username},
		Answers: items,
	}
}

func TestBuildSurveyResultsAggregates(t *testing.T) {
	survey := surveyForResults(t)
	responses := []model.ForumSurveyResponse{
		responseWith("u1", map[uint]interface{}{2: 0, 3: 5, 4: []int{0, 1}, 5: "好"}),
		responseWith("u2", map[uint]interface{}{2: 0, 3: 3, 4: []int{0, 1}, 5: "不错"}),
		responseWith("u3", map[uint]interface{}{2: 1, 3: 4, 4: []int{0, 1}}),
	}

	results := buildSurveyResults(survey, responses)
	if results["response_count"] != 3 {
		t.Fatalf("unexpected response_count: %#v", results["response_count"])
	}

	blocks, ok := results["blocks"].([]map[string]interface{})
	if !ok || len(blocks) != 5 {
		t.Fatalf("unexpected blocks: %#v", results["blocks"])
	}

	markdown := blocks[0]
	if markdown["kind"] != model.SurveyBlockMarkdown || markdown["content"] != "统计页说明" {
		t.Fatalf("unexpected markdown block: %#v", markdown)
	}
	html, _ := markdown["content_html"].(string)
	if html == "" || !strings.Contains(html, "统计页说明") {
		t.Fatalf("unexpected markdown content_html: %#v", markdown["content_html"])
	}

	single := blocks[1]
	options, ok := single["options"].([]map[string]interface{})
	if !ok || len(options) != 2 {
		t.Fatalf("unexpected single options: %#v", single["options"])
	}
	if options[0]["count"] != 2 || options[0]["percent"] != 67 {
		t.Fatalf("unexpected option 0: %#v", options[0])
	}
	if options[1]["count"] != 1 || options[1]["percent"] != 33 {
		t.Fatalf("unexpected option 1: %#v", options[1])
	}

	scale := blocks[2]
	if scale["average"] != 4.0 {
		t.Fatalf("unexpected scale average: %#v", scale["average"])
	}

	ranking := blocks[3]
	rankOptions, ok := ranking["options"].([]map[string]interface{})
	if !ok || len(rankOptions) != 2 {
		t.Fatalf("unexpected ranking options: %#v", ranking["options"])
	}
	if rankOptions[0]["average_rank"] != 1.0 || rankOptions[1]["average_rank"] != 2.0 {
		t.Fatalf("unexpected ranking averages: %#v", rankOptions)
	}

	text := blocks[4]
	texts, ok := text["texts"].([]map[string]interface{})
	if !ok || len(texts) != 2 {
		t.Fatalf("unexpected texts: %#v", text["texts"])
	}
}
