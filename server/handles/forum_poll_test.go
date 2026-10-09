package handles

import (
	"testing"

	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
)

func pollView(voted bool, canSeeResults bool, voterCount int64, counts map[uint]int64, selected map[uint]bool) *db.ForumPollView {
	return &db.ForumPollView{
		Poll: &model.ForumPoll{
			ID:       1,
			Multiple: true,
			Options: []model.ForumPollOption{
				{ID: 10, Text: "A"},
				{ID: 11, Text: "B"},
			},
		},
		Voted:         voted,
		CanSeeResults: canSeeResults,
		Selected:      selected,
		Counts:        counts,
		VoterCount:    voterCount,
	}
}

func optionByID(t *testing.T, payload map[string]interface{}, id uint) map[string]interface{} {
	t.Helper()
	options, ok := payload["options"].([]map[string]interface{})
	if !ok {
		t.Fatalf("options has unexpected type %#v", payload["options"])
	}
	for _, option := range options {
		if option["id"] == id {
			return option
		}
	}
	t.Fatalf("option %d not found", id)
	return nil
}

func TestBuildPollPayloadHidesResultsBeforeVoting(t *testing.T) {
	view := pollView(false, false, 0, nil, nil)
	payload := buildPollPayload(view)

	if payload["voted"] != false || payload["can_see_results"] != false {
		t.Fatalf("unexpected flags: %#v", payload)
	}
	if _, ok := payload["total_votes"]; ok {
		t.Fatal("total_votes must be hidden before voting")
	}

	option := optionByID(t, payload, 10)
	if _, ok := option["count"]; ok {
		t.Fatal("count must be hidden before voting")
	}
	if _, ok := option["percent"]; ok {
		t.Fatal("percent must be hidden before voting")
	}
	if _, ok := option["selected"]; ok {
		t.Fatal("selected must be hidden before voting")
	}
	if option["text"] != "A" {
		t.Fatalf("unexpected option text: %#v", option["text"])
	}
}

func TestBuildPollPayloadAuthorSeesResultsWithoutVoting(t *testing.T) {
	view := pollView(false, true, 3, map[uint]int64{10: 2, 11: 1}, nil)
	payload := buildPollPayload(view)

	if payload["voted"] != false || payload["can_see_results"] != true {
		t.Fatalf("unexpected flags: %#v", payload)
	}
	if payload["total_votes"] != int64(3) {
		t.Fatalf("unexpected total_votes: %#v", payload["total_votes"])
	}

	option := optionByID(t, payload, 10)
	if option["count"] != int64(2) {
		t.Fatalf("unexpected count: %#v", option["count"])
	}
	if option["percent"] != 67 {
		t.Fatalf("unexpected percent: %#v", option["percent"])
	}
	if _, ok := option["selected"]; ok {
		t.Fatal("selected must be hidden when the viewer has not voted")
	}
}

func TestBuildPollPayloadVoterSeesResultsAndSelection(t *testing.T) {
	view := pollView(true, true, 3, map[uint]int64{10: 1, 11: 2}, map[uint]bool{10: true})
	payload := buildPollPayload(view)

	if payload["voted"] != true || payload["can_see_results"] != true {
		t.Fatalf("unexpected flags: %#v", payload)
	}

	if optionByID(t, payload, 10)["selected"] != true {
		t.Fatal("expected option 10 to be selected")
	}
	if optionByID(t, payload, 11)["selected"] != false {
		t.Fatal("expected option 11 to be unselected")
	}
	if optionByID(t, payload, 10)["percent"] != 33 {
		t.Fatalf("unexpected percent: %#v", optionByID(t, payload, 10)["percent"])
	}
}

func TestPollPercent(t *testing.T) {
	if got := pollPercent(1, 3); got != 33 {
		t.Fatalf("pollPercent(1,3) = %d, want 33", got)
	}
	if got := pollPercent(2, 3); got != 67 {
		t.Fatalf("pollPercent(2,3) = %d, want 67", got)
	}
	if got := pollPercent(5, 0); got != 0 {
		t.Fatalf("pollPercent(5,0) = %d, want 0", got)
	}
}

func TestNormalizePollInput(t *testing.T) {
	if _, _, err := normalizePollInput(false, []string{"A"}); err == nil {
		t.Fatal("expected error for a single option")
	}

	tooMany := make([]string, maxPollOptions+1)
	for i := range tooMany {
		tooMany[i] = string(rune('A' + i))
	}
	if _, _, err := normalizePollInput(false, tooMany); err == nil {
		t.Fatal("expected error for too many options")
	}

	long := make([]rune, maxPollOptionLength+1)
	for i := range long {
		long[i] = 'x'
	}
	if _, _, err := normalizePollInput(false, []string{string(long), "B"}); err == nil {
		t.Fatal("expected error for an over-long option")
	}

	multiple, options, err := normalizePollInput(true, []string{"  A  ", "", "A", "B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !multiple {
		t.Fatal("expected multiple to be preserved")
	}
	if len(options) != 2 || options[0] != "A" || options[1] != "B" {
		t.Fatalf("unexpected normalized options: %#v", options)
	}
}

func TestResolveVoteOptionIDs(t *testing.T) {
	poll := &model.ForumPoll{
		Multiple: false,
		Options:  []model.ForumPollOption{{ID: 10}, {ID: 11}},
	}

	if _, err := resolveVoteOptionIDs(poll, []uint{10, 11}); err == nil {
		t.Fatal("expected error when a single-choice poll receives two options")
	}
	if got, err := resolveVoteOptionIDs(poll, []uint{10, 10}); err != nil || len(got) != 1 {
		t.Fatalf("expected duplicates to collapse, got %#v err %v", got, err)
	}
	if _, err := resolveVoteOptionIDs(poll, []uint{99}); err == nil {
		t.Fatal("expected error for an option outside the poll")
	}
	if _, err := resolveVoteOptionIDs(poll, nil); err == nil {
		t.Fatal("expected error for an empty selection")
	}

	multi := &model.ForumPoll{
		Multiple: true,
		Options:  []model.ForumPollOption{{ID: 10}, {ID: 11}},
	}
	if got, err := resolveVoteOptionIDs(multi, []uint{10, 11}); err != nil || len(got) != 2 {
		t.Fatalf("expected both options for a multi poll, got %#v err %v", got, err)
	}
}
