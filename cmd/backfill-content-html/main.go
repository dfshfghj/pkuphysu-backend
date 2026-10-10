package main

import (
	"flag"
	"fmt"
	"log"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "print the converted html without writing")
	flag.Parse()

	config.InitConfig()

	d := config.Conf.Database
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		d.Host, d.User, d.Password, d.DBName, d.Port, d.SSLMode)

	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: d.TablePrefix},
	})
	if err != nil {
		log.Fatalf("connect to %s: %v", d.DBName, err)
	}

	log.Printf("database=%s", d.DBName)

	backfillPosts(g, *dryRun)
	backfillComments(g, *dryRun)
	backfillProfiles(g, *dryRun)
	backfillSurveyBlocks(g, *dryRun)
}

func backfillPosts(g *gorm.DB, dryRun bool) {
	var posts []model.ForumPost
	if err := g.Order("id ASC").Find(&posts).Error; err != nil {
		log.Fatalf("query posts: %v", err)
	}

	log.Printf("found %d post(s)", len(posts))
	for _, post := range posts {
		html := utils.MarkdownToHtml(post.Content)
		text := utils.MarkdownToText(post.Content)
		if dryRun {
			log.Printf("[dry-run] post %d: %d chars -> %d bytes html", post.ID, len(post.Content), len(html))
			continue
		}
		if err := g.Model(&model.ForumPost{}).Where("id = ?", post.ID).
			Updates(map[string]interface{}{"content_html": html, "content_text": text}).Error; err != nil {
			log.Fatalf("update post %d: %v", post.ID, err)
		}
	}
	log.Printf("posts done: %d", len(posts))
}

func backfillComments(g *gorm.DB, dryRun bool) {
	var comments []model.ForumComment
	if err := g.Order("id ASC").Find(&comments).Error; err != nil {
		log.Fatalf("query comments: %v", err)
	}

	log.Printf("found %d comment(s)", len(comments))
	for _, comment := range comments {
		html := utils.MarkdownToHtml(comment.Content)
		text := utils.MarkdownToText(comment.Content)
		if dryRun {
			log.Printf("[dry-run] comment %d: %d chars -> %d bytes html", comment.ID, len(comment.Content), len(html))
			continue
		}
		if err := g.Model(&model.ForumComment{}).Where("id = ?", comment.ID).
			Updates(map[string]interface{}{"content_html": html, "content_text": text}).Error; err != nil {
			log.Fatalf("update comment %d: %v", comment.ID, err)
		}
	}
	log.Printf("comments done: %d", len(comments))
}

func backfillProfiles(g *gorm.DB, dryRun bool) {
	var profiles []model.UserProfile
	if err := g.Order("id ASC").Find(&profiles).Error; err != nil {
		log.Fatalf("query profiles: %v", err)
	}

	log.Printf("found %d profile(s)", len(profiles))
	for _, profile := range profiles {
		html := utils.MarkdownToHtml(profile.Content)
		if dryRun {
			log.Printf("[dry-run] profile %d: %d chars -> %d bytes html", profile.ID, len(profile.Content), len(html))
			continue
		}
		if err := g.Model(&model.UserProfile{}).Where("id = ?", profile.ID).
			Update("content_html", html).Error; err != nil {
			log.Fatalf("update profile %d: %v", profile.ID, err)
		}
	}
	log.Printf("profiles done: %d", len(profiles))
}

func backfillSurveyBlocks(g *gorm.DB, dryRun bool) {
	var blocks []model.ForumSurveyBlock
	if err := g.Where("kind = ?", model.SurveyBlockMarkdown).
		Order("id ASC").
		Find(&blocks).Error; err != nil {
		log.Fatalf("query survey blocks: %v", err)
	}

	log.Printf("found %d survey markdown block(s)", len(blocks))
	for _, block := range blocks {
		html := utils.MarkdownToHtml(block.Content)
		if dryRun {
			log.Printf("[dry-run] survey block %d: %d chars -> %d bytes html", block.ID, len(block.Content), len(html))
			continue
		}
		if err := g.Model(&model.ForumSurveyBlock{}).Where("id = ?", block.ID).
			Update("content_html", html).Error; err != nil {
			log.Fatalf("update survey block %d: %v", block.ID, err)
		}
	}
	log.Printf("survey blocks done: %d", len(blocks))
}
