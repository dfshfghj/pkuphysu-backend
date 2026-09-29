package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestForumPostTagColumnMapping(t *testing.T) {
	parsedSchema, err := schema.Parse(&ForumPostTag{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	postIDField := parsedSchema.LookUpField("PostID")
	if postIDField == nil {
		t.Fatal("PostID field not found")
	}
	if postIDField.DBName != "forum_post_id" {
		t.Fatalf("PostID DBName = %q, want %q", postIDField.DBName, "forum_post_id")
	}

	tagIDField := parsedSchema.LookUpField("TagID")
	if tagIDField == nil {
		t.Fatal("TagID field not found")
	}
	if tagIDField.DBName != "forum_tag_id" {
		t.Fatalf("TagID DBName = %q, want %q", tagIDField.DBName, "forum_tag_id")
	}
}
