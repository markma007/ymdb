package ymdb

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	if err := InitiateDBM(t.TempDir() + "/ymdb.sqlite"); err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
		dbMu.Lock()
		DB, DBM = nil, nil
		dbMu.Unlock()
	})
}

func TestOptionsLifecycle(t *testing.T) {
	setupTestDB(t)
	first, err := OptionNewE("theme", "dark", "app", MetaTypeString)
	if err != nil {
		t.Fatalf("create option: %v", err)
	}
	if _, err := OptionNewE("theme", "light", "app", MetaTypeString); err == nil {
		t.Fatal("expected duplicate group/key to fail")
	}
	if err := first.SetMeta("source", "user", MetaTypeString); err != nil {
		t.Fatalf("set option metadata: %v", err)
	}
	meta, err := first.MetaMap()
	if err != nil || meta["source"].Value != "user" {
		t.Fatalf("unexpected metadata: %#v, %v", meta, err)
	}
	groups, err := OptionGroupNamesE()
	if err != nil || len(groups) != 1 || groups[0] != "app" {
		t.Fatalf("unexpected groups: %#v, %v", groups, err)
	}
}

func TestOptionsAreIsolatedByGroup(t *testing.T) {
	setupTestDB(t)
	appTheme, err := OptionSet("", "theme", "dark", MetaTypeString)
	if err != nil {
		t.Fatal(err)
	}
	pluginTheme, err := OptionSet(" plugin.gallery ", "theme", "light", MetaTypeString)
	if err != nil {
		t.Fatal(err)
	}
	if appTheme.Group != DefaultOptionGroup || pluginTheme.Group != "plugin.gallery" {
		t.Fatalf("groups were not normalized: %q, %q", appTheme.Group, pluginTheme.Group)
	}
	if _, err := OptionSet("app", "theme", "system", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	app, _ := OptionGet("app", "theme")
	plugin, _ := OptionGet("plugin.gallery", "theme")
	if app.Value != "system" || plugin.Value != "light" {
		t.Fatalf("group values leaked: app=%q plugin=%q", app.Value, plugin.Value)
	}
	config, err := OptionMap("plugin.gallery")
	if err != nil || len(config) != 1 || config["theme"].Value != "light" {
		t.Fatalf("unexpected group map: %#v, %v", config, err)
	}
	groups, _ := OptionGroupNamesE()
	if len(groups) != 2 || groups[0] != "app" || groups[1] != "plugin.gallery" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
	if err := OptionDeleteGroup("plugin.gallery"); err != nil {
		t.Fatal(err)
	}
	if _, err := OptionGet("plugin.gallery", "theme"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted group remains: %v", err)
	}
	if _, err := OptionGet("app", "theme"); err != nil {
		t.Fatalf("deleting one group affected another: %v", err)
	}
}

func TestOptionCanBeRecreatedAfterDelete(t *testing.T) {
	setupTestDB(t)
	if _, err := OptionSet("plugin.cache", "enabled", "true", MetaTypeBool); err != nil {
		t.Fatal(err)
	}
	if err := OptionDelete("plugin.cache", "enabled"); err != nil {
		t.Fatal(err)
	}
	recreated, err := OptionSet("plugin.cache", "enabled", "false", MetaTypeBool)
	if err != nil {
		t.Fatal(err)
	}
	if recreated.Value != "false" {
		t.Fatalf("option was not recreated: %#v", recreated)
	}
}

func TestSQLiteConfiguration(t *testing.T) {
	setupTestDB(t)
	var foreignKeys, busyTimeout int
	var journalMode string
	if err := DB.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 || journalMode != "wal" {
		t.Fatalf("sqlite pragmas: fk=%d busy=%d journal=%q", foreignKeys, busyTimeout, journalMode)
	}
}

func TestMetadataForeignKeysAndSetUniqueness(t *testing.T) {
	setupTestDB(t)
	if err := DB.Create(&UserMeta{UserID: 999999, Key: "orphan", Value: "x", Type: MetaTypeString}).Error; err == nil {
		t.Fatal("expected orphan user metadata to fail")
	}
	if err := DB.Create(&OptionMeta{OptionID: 999999, Key: "orphan", Value: "x", Type: MetaTypeString}).Error; err == nil {
		t.Fatal("expected orphan option metadata to fail")
	}
	if err := DB.Create(&PostMeta{PostID: 999999, Key: "orphan", Value: "x", Type: MetaTypeString}).Error; err == nil {
		t.Fatal("expected orphan post metadata to fail")
	}
	u, err := CreateUser("unique-meta", "unique@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if err := u.SetMeta("role", "reader", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	if err := u.SetMeta("role", "editor", MetaTypeString); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := DB.Model(&UserMeta{}).Where("user_id = ? AND key = ?", u.ID, "role").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("SetMeta created %d rows", count)
	}
}

func TestManagerActivateAndClose(t *testing.T) {
	dbMu.Lock()
	DB, DBM = nil, nil
	dbMu.Unlock()
	manager, err := Open(t.TempDir() + "/manager.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewPostE("before-activate"); err == nil {
		t.Fatal("package API unexpectedly used inactive manager")
	}
	if err := manager.Activate(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPostE("active"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPostE("after-close"); err == nil {
		t.Fatal("package API remained active after close")
	}
}

func TestPostAndUserMetadata(t *testing.T) {
	setupTestDB(t)
	p := NewPost("article")
	if p.ID == 0 {
		t.Fatal("post was not saved")
	}
	if err := p.SetMeta("views", "12", MetaTypeInt); err != nil {
		t.Fatal(err)
	}
	var views int
	m, err := p.FindMeta("views")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Decode(&views); err != nil || views != 12 {
		t.Fatalf("decoded views=%d err=%v", views, err)
	}
	child, err := p.AddChildSameType()
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentID == nil || *child.ParentID != p.ID {
		t.Fatal("child relation was not persisted")
	}
	u, err := CreateUser("alex", "Alex@Example.com", "already-hashed")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "alex@example.com" {
		t.Fatalf("email was not normalized: %s", u.Email)
	}
	if err := u.SetMeta("timezone", "UTC", MetaTypeString); err != nil {
		t.Fatal(err)
	}
}
