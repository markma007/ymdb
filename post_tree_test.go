package ymdb

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestTreeNavigationAndOrdering(t *testing.T) {
	setupTestDB(t)
	root := NewPost("page")
	first, err := root.AddChildSameType()
	if err != nil {
		t.Fatal(err)
	}
	second, err := first.AddNext()
	if err != nil {
		t.Fatal(err)
	}
	grandchild, err := first.AddChildSameType()
	if err != nil {
		t.Fatal(err)
	}
	children, err := root.ChildPosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 || children[0].ID != first.ID || children[1].ID != second.ID {
		t.Fatalf("wrong children order: %#v", children)
	}
	if err := first.MoveTo(root, -1); err != nil {
		t.Fatal(err)
	}
	children, _ = root.ChildPosts()
	if children[0].ID != second.ID || children[1].ID != first.ID || children[0].Position != 0 || children[1].Position != 1 {
		t.Fatalf("append move did not compact positions: %#v", children)
	}
	if err := first.MoveTo(root, 0); err != nil {
		t.Fatal(err)
	}
	parent, err := second.ParentPost()
	if err != nil || parent.ID != root.ID {
		t.Fatalf("wrong parent: %#v, %v", parent, err)
	}
	next, err := first.NextSibling()
	if err != nil || next.ID != second.ID {
		t.Fatalf("wrong next sibling: %#v, %v", next, err)
	}
	previous, err := second.PreviousSibling()
	if err != nil || previous.ID != first.ID {
		t.Fatalf("wrong previous sibling: %#v, %v", previous, err)
	}
	ancestors, err := grandchild.Ancestors()
	if err != nil || len(ancestors) != 2 || ancestors[0].ID != first.ID || ancestors[1].ID != root.ID {
		t.Fatalf("wrong ancestors: %#v, %v", ancestors, err)
	}
	descendants, err := root.Descendants()
	if err != nil || len(descendants) != 3 {
		t.Fatalf("wrong descendants: %#v, %v", descendants, err)
	}
	if err := root.MoveTo(grandchild, -1); err == nil {
		t.Fatal("expected cycle prevention")
	}
}

func TestMoveDetachAndDeleteSubtree(t *testing.T) {
	setupTestDB(t)
	left := NewPost("page")
	right := NewPost("page")
	child, _ := left.AddChildSameType()
	grandchild, _ := child.AddChildSameType()
	if err := child.MoveTo(right, 0); err != nil {
		t.Fatal(err)
	}
	if children, _ := left.ChildPosts(); len(children) != 0 {
		t.Fatalf("old parent retained child: %#v", children)
	}
	if err := child.Detach(); err != nil {
		t.Fatal(err)
	}
	if child.ParentID != nil {
		t.Fatal("detached node still has parent")
	}
	if err := grandchild.SetMeta("test", "value", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	if err := child.DeleteSubtree(); err != nil {
		t.Fatal(err)
	}
	if _, err := grandchild.FindMeta("test"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("subtree metadata remains: %v", err)
	}
	var found Post
	if err := DB.First(&found, grandchild.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("descendant remains: %v", err)
	}
}
