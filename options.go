package ymdb

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DefaultOptionGroup = "app"

type OptionModel struct {
	gorm.Model
	Group string `json:"group" gorm:"not null;default:app;uniqueIndex:idx_option_group_key,priority:1"`
	Key   string `json:"key" gorm:"not null;uniqueIndex:idx_option_group_key,priority:2"`
	Value string `json:"value" gorm:"type:text"`
}

func OptionNewE(key, value, group string) (OptionModel, error) {
	db, err := defaultDB()
	if err != nil {
		return OptionModel{}, err
	}
	key = strings.TrimSpace(key)
	group = normalizeOptionGroup(group)
	if key == "" {
		return OptionModel{}, errors.New("ymdb: option key is required")
	}
	opt := OptionModel{Group: group, Key: key, Value: value}
	err = db.Create(&opt).Error
	return opt, err
}

func normalizeOptionGroup(group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		return DefaultOptionGroup
	}
	return group
}
func OptionNew(key, value, group string) OptionModel {
	opt, _ := OptionNewE(key, value, group)
	return opt
}

func OptionGroupNamesE() ([]string, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var groups []string
	err = db.Model(&OptionModel{}).Distinct("group").Order("`group`").Pluck("group", &groups).Error
	return groups, err
}
func OptionGroupNames() []string { v, _ := OptionGroupNamesE(); return v }
func OptionQueryByGroupE(group string) ([]OptionModel, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []OptionModel
	err = db.Where("`group` = ?", normalizeOptionGroup(group)).Order("`key`, id").Find(&rows).Error
	return rows, err
}

// OptionGet retrieves a key within a group. The same key may exist in other groups.
func OptionGet(group, key string) (OptionModel, error) {
	db, err := defaultDB()
	if err != nil {
		return OptionModel{}, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return OptionModel{}, errors.New("ymdb: option key is required")
	}
	var option OptionModel
	err = db.Where("`group` = ? AND `key` = ?", normalizeOptionGroup(group), key).First(&option).Error
	return option, err
}

// OptionSet creates or replaces one option identified by its group/key pair.
func OptionSet(group, key, value string) (OptionModel, error) {
	db, err := defaultDB()
	if err != nil {
		return OptionModel{}, err
	}
	key = strings.TrimSpace(key)
	group = normalizeOptionGroup(group)
	if key == "" {
		return OptionModel{}, errors.New("ymdb: option key is required")
	}
	option := OptionModel{Group: group, Key: key, Value: value}
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"value": value, "deleted_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}),
	}).Create(&option).Error
	if err != nil {
		return OptionModel{}, err
	}
	err = db.Where("`group` = ? AND `key` = ?", group, key).First(&option).Error
	return option, err
}

// OptionMap returns a group's configuration keyed by option key.
func OptionMap(group string) (map[string]string, error) {
	options, err := OptionQueryByGroupE(group)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(options))
	for _, option := range options {
		result[option.Key] = option.Value
	}
	return result, nil
}

// OptionDelete removes one group-scoped option.
func OptionDelete(group, key string) error {
	option, err := OptionGet(group, key)
	if err != nil {
		return err
	}
	return deleteOption(option)
}

// OptionDeleteGroup removes a complete configuration namespace atomically.
func OptionDeleteGroup(group string) error {
	db, err := defaultDB()
	if err != nil {
		return err
	}
	group = normalizeOptionGroup(group)
	return db.Unscoped().Where("`group` = ?", group).Delete(&OptionModel{}).Error
}
func OptionQueryByGroup(group string) []OptionModel { v, _ := OptionQueryByGroupE(group); return v }
func OptionGetByID(id int) OptionModel {
	db, err := defaultDB()
	if err != nil {
		return OptionModel{}
	}
	var row OptionModel
	_ = db.First(&row, id).Error
	return row
}

func OptionUpdateByID(id int, key, value string) {
	db, err := defaultDB()
	if err != nil {
		return
	}
	updates := map[string]any{}
	if key != "" {
		updates["key"] = key
	}
	if value != "" {
		updates["value"] = value
	}
	if len(updates) > 0 {
		_ = db.Model(&OptionModel{}).Where("id = ?", id).Updates(updates).Error
	}
}
func OptionDeleteByID(id int) OptionModel {
	db, err := defaultDB()
	if err != nil {
		return OptionModel{}
	}
	var opt OptionModel
	if db.First(&opt, id).Error == nil {
		_ = deleteOption(opt)
	}
	return opt
}

func deleteOption(opt OptionModel) error {
	db, err := defaultDB()
	if err != nil {
		return err
	}
	return db.Unscoped().Delete(&opt).Error
}
