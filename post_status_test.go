package ymdb

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestPostStatusLifecycle(t *testing.T) {
	setupTestDB(t)
	post, err := NewPostE("article")
	if err != nil {
		t.Fatal(err)
	}
	if post.Status != PostStatusDraft {
		t.Fatalf("new status=%q want=%q", post.Status, PostStatusDraft)
	}
	if err := post.Publish(); err != nil {
		t.Fatal(err)
	}
	if post.Status != PostStatusPublish {
		t.Fatalf("published status=%q", post.Status)
	}
	loaded, err := FindPostByID(int(post.ID))
	if err != nil || loaded.Status != PostStatusPublish {
		t.Fatalf("published post=%#v err=%v", loaded, err)
	}
	if err := post.Delete(); err != nil {
		t.Fatal(err)
	}
	if post.Status != PostStatusDeleted {
		t.Fatalf("deleted status=%q", post.Status)
	}
	if _, err := FindPostByID(int(post.ID)); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted post remained queryable: %v", err)
	}
	var raw Post
	if err := DB.Unscoped().First(&raw, post.ID).Error; err != nil {
		t.Fatal(err)
	}
	if raw.Status != PostStatusDeleted || !raw.DeletedAt.Valid {
		t.Fatalf("soft deletion not persisted: %#v", raw)
	}
}

func TestPostQueriesExcludeDeletedStatus(t *testing.T) {
	setupTestDB(t)
	visible, _ := NewPostE("article")
	deleted, _ := NewPostE("article")
	legacy, _ := NewPostE("article")
	if err := DB.Model(deleted).Update("status", PostStatusDeleted).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Model(legacy).Update("status", "delete").Error; err != nil {
		t.Fatal(err)
	}
	posts, err := QueryPostsE("post_type = ?", "article")
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].ID != visible.ID {
		t.Fatalf("deleted statuses leaked into query: %#v", posts)
	}
	roots, err := RootPosts("article")
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].ID != visible.ID {
		t.Fatalf("deleted statuses leaked into roots: %#v", roots)
	}
}

func TestNewAttachmentStartsDraft(t *testing.T) {
	setupTestDB(t)
	parent, _ := NewPostE("article")
	attachment, err := parent.AddAttachment()
	if err != nil {
		t.Fatal(err)
	}
	if attachment.Status != PostStatusDraft {
		t.Fatalf("attachment status=%q", attachment.Status)
	}
}
