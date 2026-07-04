package ymdb

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"gorm.io/gorm"
)

//go:embed fixtures/default.json
var fixtureFiles embed.FS

type Fixtures struct {
	Options []OptionFixture `json:"options"`
}

type OptionFixture struct {
	Group string `json:"group"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// InstallDefaultFixtures installs embedded application defaults without
// replacing existing or intentionally soft-deleted options.
func InstallDefaultFixtures(db *gorm.DB) error {
	file, err := fixtureFiles.Open("fixtures/default.json")
	if err != nil {
		return fmt.Errorf("ymdb: open default fixtures: %w", err)
	}
	defer file.Close()
	return InstallFixtures(db, file)
}

// InstallFixtures installs a JSON fixture stream idempotently. The group/key
// pair is the identity of an option; existing values always win.
func InstallFixtures(db *gorm.DB, reader io.Reader) error {
	if db == nil {
		return errors.New("ymdb: fixture database is required")
	}
	if reader == nil {
		return errors.New("ymdb: fixture reader is required")
	}
	decoder := json.NewDecoder(io.LimitReader(reader, 4<<20))
	decoder.DisallowUnknownFields()
	var fixtures Fixtures
	if err := decoder.Decode(&fixtures); err != nil {
		return fmt.Errorf("ymdb: decode fixtures: %w", err)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for index, fixture := range fixtures.Options {
			group := normalizeOptionGroup(fixture.Group)
			key := strings.TrimSpace(fixture.Key)
			if key == "" {
				return fmt.Errorf("ymdb: fixture option %d has an empty key", index)
			}
			var existing OptionModel
			err := tx.Unscoped().Where("`group` = ? AND `key` = ?", group, key).First(&existing).Error
			if err == nil {
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			option := OptionModel{Group: group, Key: key, Value: fixture.Value, Type: normalizeMetaType(fixture.Type)}
			if err := tx.Create(&option).Error; err != nil {
				return fmt.Errorf("ymdb: install fixture %s/%s: %w", group, key, err)
			}
		}
		return nil
	})
}
