package ymdb

import (
	"errors"
	"fmt"
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
	Type  string `json:"type" gorm:"not null;default:string"`
}

type OptionMeta struct {
	gorm.Model
	OptionID uint         `json:"option_id" gorm:"not null;uniqueIndex:idx_option_meta_lookup,priority:1"`
	Option   *OptionModel `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Key      string       `json:"key" gorm:"not null;uniqueIndex:idx_option_meta_lookup,priority:2"`
	Value    string       `json:"value" gorm:"type:text"`
	Type     string       `json:"type" gorm:"not null;default:string"`
}

func OptionNewE(key, value, group, valueType string) (OptionModel, error) {
	db, err := defaultDB()
	if err != nil {
		return OptionModel{}, err
	}
	key = strings.TrimSpace(key)
	group = normalizeOptionGroup(group)
	if key == "" {
		return OptionModel{}, errors.New("ymdb: option key is required")
	}
	opt := OptionModel{Group: group, Key: key, Value: value, Type: normalizeMetaType(valueType)}
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
func OptionNew(key, value, group, valueType string) OptionModel {
	opt, _ := OptionNewE(key, value, group, valueType)
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
func OptionSet(group, key, value, valueType string) (OptionModel, error) {
	db, err := defaultDB()
	if err != nil {
		return OptionModel{}, err
	}
	key = strings.TrimSpace(key)
	group = normalizeOptionGroup(group)
	if key == "" {
		return OptionModel{}, errors.New("ymdb: option key is required")
	}
	option := OptionModel{Group: group, Key: key, Value: value, Type: normalizeMetaType(valueType)}
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"value": value, "type": option.Type, "deleted_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}),
	}).Create(&option).Error
	if err != nil {
		return OptionModel{}, err
	}
	err = db.Where("`group` = ? AND `key` = ?", group, key).First(&option).Error
	return option, err
}

// OptionMap returns a group's configuration keyed by option key.
func OptionMap(group string) (map[string]Meta, error) {
	options, err := OptionQueryByGroupE(group)
	if err != nil {
		return nil, err
	}
	result := make(map[string]Meta, len(options))
	for _, option := range options {
		result[option.Key] = Meta{Value: option.Value, Type: normalizeMetaType(option.Type)}
	}
	return result, nil
}

// OptionDelete removes one group-scoped option and its metadata.
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
	return db.Transaction(func(tx *gorm.DB) error {
		var ids []uint
		if err := tx.Model(&OptionModel{}).Where("`group` = ?", group).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := tx.Where("option_id IN ?", ids).Delete(&OptionMeta{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("`group` = ?", group).Delete(&OptionModel{}).Error; err != nil {
			return fmt.Errorf("delete option group %q: %w", group, err)
		}
		return nil
	})
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

func OptionUpdateByID(id int, key, value, valueType string) {
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
	if valueType != "" {
		updates["type"] = normalizeMetaType(valueType)
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
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("option_id = ?", opt.ID).Delete(&OptionMeta{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&opt).Error
	})
}

func (o *OptionModel) SetMeta(key, value, valueType string) error {
	db, err := defaultDB()
	if err != nil {
		return err
	}
	if o.ID == 0 || key == "" {
		return errors.New("ymdb: saved option and metadata key are required")
	}
	row := OptionMeta{OptionID: o.ID, Key: key, Value: value, Type: normalizeMetaType(valueType)}
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "option_id"}, {Name: "key"}}, DoUpdates: clause.Assignments(map[string]any{"value": value, "type": row.Type, "deleted_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})}).Create(&row).Error
}

func (o *OptionModel) MetaMap() (map[string]Meta, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []OptionMeta
	if err = db.Where("option_id = ?", o.ID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]Meta{}
	for _, r := range rows {
		out[r.Key] = Meta{Value: r.Value, Type: normalizeMetaType(r.Type)}
	}
	return out, nil
}

// MetaValueMap preserves every option metadata row, including repeated keys.
func (o *OptionModel) MetaValueMap() (map[string][]Meta, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []OptionMeta
	if err := db.Where("option_id = ?", o.ID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string][]Meta)
	for _, row := range rows {
		result[row.Key] = append(result[row.Key], Meta{Value: row.Value, Type: normalizeMetaType(row.Type)})
	}
	return result, nil
}
