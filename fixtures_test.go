package ymdb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultFixturesInstalledAndNotOverwritten(t *testing.T) {
	setupTestDB(t)
	name, err := OptionGet("app", "name")
	if err != nil {
		t.Fatal(err)
	}
	title, err := OptionGet("app", "title")
	if err != nil {
		t.Fatal(err)
	}
	root, err := OptionGet("app", "upload_root")
	if err != nil {
		t.Fatal(err)
	}
	if name.Value != "my_app" || title.Value != "My App" || root.Value != "~/{app_name}_uploads" {
		t.Fatalf("wrong defaults: %#v %#v %#v", name, title, root)
	}
	if _, err := OptionSet("app", "name", "custom_app", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	if err := InstallDefaultFixtures(DB); err != nil {
		t.Fatal(err)
	}
	name, _ = OptionGet("app", "name")
	if name.Value != "custom_app" {
		t.Fatalf("fixtures overwrote setting: %#v", name)
	}
}

func TestCustomFixturesAndUploadRootExpansion(t *testing.T) {
	setupTestDB(t)
	custom := `{"options":[{"group":"plugin.search","key":"enabled","value":"true","type":"bool"}]}`
	if err := InstallFixtures(DB, strings.NewReader(custom)); err != nil {
		t.Fatal(err)
	}
	option, err := OptionGet("plugin.search", "enabled")
	if err != nil || option.Type != MetaTypeBool {
		t.Fatalf("custom fixture=%#v err=%v", option, err)
	}
	if _, err := OptionSet("app", "name", "fixture app", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	root, err := ConfiguredUploadRoot()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "fixture-app_uploads")
	if root != want {
		t.Fatalf("expanded root=%q want=%q", root, want)
	}
}
