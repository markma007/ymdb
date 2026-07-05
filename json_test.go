package ymdb

import (
	"encoding/json"
	"strings"
	"testing"
)

func decodeJSON(t *testing.T, value string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, value)
	}
	return decoded
}

func TestPostToJSON(t *testing.T) {
	setupTestDB(t)
	p := NewPost("article")
	p.Title = "Hello"
	if err := p.SaveE(); err != nil {
		t.Fatal(err)
	}
	if err := p.SetMeta("views", "42", MetaTypeInt); err != nil {
		t.Fatal(err)
	}
	value, err := p.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeJSON(t, value)
	if decoded["post_type"] != "article" || decoded["title"] != "Hello" {
		t.Fatalf("wrong post JSON: %s", value)
	}
	meta := decoded["meta"].(map[string]any)
	if meta["views"].(map[string]any)["type"] != MetaTypeInt {
		t.Fatalf("metadata missing: %s", value)
	}
	if _, exists := decoded["DeletedAt"]; exists {
		t.Fatalf("GORM internals leaked: %s", value)
	}
}

func TestPostToDeepJSONPreservesRepeatedMeta(t *testing.T) {
	setupTestDB(t)
	p := NewPost("article")
	if _, err := p.AddMeta("tag", "go", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	if _, err := p.AddMeta("tag", "sqlite", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	value, err := p.ToDeepJSON()
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeJSON(t, value)
	post := decoded["post"].(map[string]any)
	if post["id"].(float64) != float64(p.ID) {
		t.Fatalf("post missing from deep JSON: %s", value)
	}
	meta := decoded["meta"].(map[string]any)
	tags := meta["tag"].([]any)
	if len(tags) != 2 || tags[0].(map[string]any)["value"] != "go" || tags[1].(map[string]any)["value"] != "sqlite" {
		t.Fatalf("repeated metadata lost: %s", value)
	}
	if byID := ToDeepJsonString(int(p.ID)); byID == "" {
		t.Fatal("ID-based compatibility helper returned empty JSON")
	}
}

func TestOptionAndUserToJSON(t *testing.T) {
	setupTestDB(t)
	option, err := OptionSet("plugin.gallery", "enabled", "true")
	if err != nil {
		t.Fatal(err)
	}
	optionValue, err := option.ToJson()
	if err != nil {
		t.Fatal(err)
	}
	optionJSON := decodeJSON(t, optionValue)
	if optionJSON["group"] != "plugin.gallery" || optionJSON["key"] != "enabled" {
		t.Fatalf("group identity missing: %s", optionValue)
	}
	if _, exists := optionJSON["meta"]; exists {
		t.Fatalf("option metadata leaked into JSON: %s", optionValue)
	}
	if _, exists := optionJSON["type"]; exists {
		t.Fatalf("option type leaked into JSON: %s", optionValue)
	}

	user, err := CreateUser("alex", "alex@example.com", "super-secret-hash")
	if err != nil {
		t.Fatal(err)
	}
	if err := user.SetMeta("timezone", "UTC", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	userValue, err := user.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	userJSON := decodeJSON(t, userValue)
	if userJSON["username"] != "alex" {
		t.Fatalf("wrong user JSON: %s", userValue)
	}
	if strings.Contains(userValue, "super-secret-hash") || strings.Contains(userValue, "password") {
		t.Fatalf("password leaked: %s", userValue)
	}
}

func TestUserDeepJSON(t *testing.T) {
	setupTestDB(t)
	user, err := CreateUser("sam", "sam@example.com", "hidden-hash")
	if err != nil {
		t.Fatal(err)
	}
	if err := user.SetMeta("role", "editor", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	if err := user.SetMeta("role", "reviewer", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	value, err := user.ToDeepJSON()
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeJSON(t, value)
	roles := decoded["meta"].(map[string]any)["role"].([]any)
	if len(roles) != 1 || roles[0].(map[string]any)["value"] != "reviewer" {
		t.Fatalf("user metadata lost: %s", value)
	}
	if strings.Contains(value, "hidden-hash") || strings.Contains(value, "password") {
		t.Fatalf("password leaked: %s", value)
	}
	if UserToDeepJsonString(int(user.ID)) == "" {
		t.Fatal("user ID helper returned empty JSON")
	}
}

func TestBulkJSON(t *testing.T) {
	setupTestDB(t)
	first := NewPost("article")
	second := NewPost("article")
	if err := first.SetMeta("views", "1", MetaTypeInt); err != nil {
		t.Fatal(err)
	}
	if err := second.SetMeta("views", "2", MetaTypeInt); err != nil {
		t.Fatal(err)
	}
	value, err := PostsToJSON([]Post{*first, *second})
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 || decoded[0]["meta"].(map[string]any)["views"].(map[string]any)["value"] != "1" || decoded[1]["meta"].(map[string]any)["views"].(map[string]any)["value"] != "2" {
		t.Fatalf("wrong bulk JSON: %s", value)
	}
	empty, err := PostsToJSON(nil)
	if err != nil || empty != "[]" {
		t.Fatalf("empty bulk JSON=%q err=%v", empty, err)
	}
}
