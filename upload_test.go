package ymdb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUploadCreatesAttachmentAndDatedFile(t *testing.T) {
	setupTestDB(t)
	root := t.TempDir()
	parent := NewPost("article")
	result, err := Upload(strings.NewReader("hello upload"), UploadConfig{Root: root, Filename: "../My File.txt", Parent: parent, Now: time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Post.PostType != "attachment" || result.Post.ParentID == nil || *result.Post.ParentID != parent.ID {
		t.Fatalf("invalid attachment: %#v", result.Post)
	}
	if result.Post.Mime != PostMimeText || result.Post.Format != PostFormatText {
		t.Fatalf("invalid file type: %#v", result.Post)
	}
	if !strings.HasPrefix(result.RelativePath, "2026/07/04/") {
		t.Fatalf("wrong dated path: %s", result.RelativePath)
	}
	content, err := os.ReadFile(result.AbsolutePath)
	if err != nil || string(content) != "hello upload" {
		t.Fatalf("stored content=%q err=%v", content, err)
	}
	if filepath.Base(result.AbsolutePath) == "My File.txt" || strings.Contains(filepath.Base(result.AbsolutePath), " ") {
		t.Fatalf("filename was not sanitized/uniquified: %s", result.AbsolutePath)
	}
	meta, err := result.Post.MetaMap()
	if err != nil {
		t.Fatal(err)
	}
	if meta["size_bytes"].Value != "12" || meta["sha256"].Value != result.SHA256 || meta["file_mime"].Value != "text/plain; charset=utf-8" {
		t.Fatalf("upload metadata missing: %#v", meta)
	}
}

func TestUploadRejectsOversizedFileAndCleansUp(t *testing.T) {
	setupTestDB(t)
	root := t.TempDir()
	_, err := Upload(strings.NewReader("too large"), UploadConfig{Root: root, Filename: "file.txt", MaxBytes: 3, Now: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)})
	if err == nil {
		t.Fatal("expected size limit error")
	}
	var attachments int64
	if err := DB.Model(&Post{}).Where("post_type = ?", "attachment").Count(&attachments).Error; err != nil {
		t.Fatal(err)
	}
	if attachments != 0 {
		t.Fatalf("attachment persisted after failed upload: %d", attachments)
	}
}

func TestUploadUsesConfiguredOptionRoot(t *testing.T) {
	setupTestDB(t)
	root := t.TempDir()
	option, err := SetUploadRoot("  " + root + "  ")
	if err != nil {
		t.Fatal(err)
	}
	if option.Group != "app" || option.Key != "upload_root" {
		t.Fatalf("wrong upload option: %#v", option)
	}
	parent := NewPost("article")
	result, err := parent.Upload(strings.NewReader("configured"), "configured.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	if !pathWithinRoot(root, result.AbsolutePath) {
		t.Fatalf("upload escaped configured root: %s", result.AbsolutePath)
	}
	if _, err := os.Stat(result.AbsolutePath); err != nil {
		t.Fatalf("configured upload missing: %v", err)
	}
}

func TestDeleteSubtreeRemovesAttachmentFile(t *testing.T) {
	setupTestDB(t)
	root := t.TempDir()
	parent := NewPost("article")
	result, err := parent.Upload(strings.NewReader("temporary"), "temporary.txt", root)
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.DeleteSubtree(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(result.AbsolutePath); !os.IsNotExist(err) {
		t.Fatalf("attachment file remains after subtree deletion: %v", err)
	}
	var count int64
	if err := DB.Model(&Post{}).Where("id = ?", result.Post.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("attachment post remains after subtree deletion: %d", count)
	}
}
