package ymdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

const (
	MetaTypeString = "string"
	MetaTypeInt    = "int"
	MetaTypeFloat  = "float"
	MetaTypeBool   = "bool"
	MetaTypeJSON   = "json"
)

type PostMeta struct {
	gorm.Model
	PostID uint   `json:"post_id" gorm:"not null;index:idx_post_meta_lookup,priority:1"`
	Post   *Post  `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Key    string `json:"key" gorm:"not null;index:idx_post_meta_lookup,priority:2"`
	Value  string `json:"value" gorm:"type:text"`
	Type   string `json:"type" gorm:"not null;default:string"`
}

type Meta struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type MetaSlice []Meta

func normalizeMetaType(t string) string {
	if t == "" || t == "str" {
		return MetaTypeString
	}
	return t
}

func (m Meta) Decode(dst any) error {
	switch normalizeMetaType(m.Type) {
	case MetaTypeString:
		p, ok := dst.(*string)
		if !ok {
			return errors.New("string metadata requires *string")
		}
		*p = m.Value
		return nil
	case MetaTypeInt:
		p, ok := dst.(*int)
		if !ok {
			return errors.New("int metadata requires *int")
		}
		v, err := strconv.Atoi(m.Value)
		if err == nil {
			*p = v
		}
		return err
	case MetaTypeFloat:
		p, ok := dst.(*float64)
		if !ok {
			return errors.New("float metadata requires *float64")
		}
		v, err := strconv.ParseFloat(m.Value, 64)
		if err == nil {
			*p = v
		}
		return err
	case MetaTypeBool:
		p, ok := dst.(*bool)
		if !ok {
			return errors.New("bool metadata requires *bool")
		}
		v, err := strconv.ParseBool(m.Value)
		if err == nil {
			*p = v
		}
		return err
	case MetaTypeJSON:
		return json.Unmarshal([]byte(m.Value), dst)
	default:
		return fmt.Errorf("unknown metadata type %q", m.Type)
	}
}

func (p *Post) AddMeta(key, value, valueType string) (*PostMeta, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	if p.ID == 0 {
		return nil, errors.New("ymdb: save post before adding metadata")
	}
	if key == "" {
		return nil, errors.New("ymdb: metadata key is required")
	}
	pm := &PostMeta{PostID: p.ID, Key: key, Value: value, Type: normalizeMetaType(valueType)}
	if err := db.Create(pm).Error; err != nil {
		return nil, fmt.Errorf("create post metadata: %w", err)
	}
	return pm, nil
}

func (p *Post) SetMeta(key, value, valueType string) error {
	db, err := defaultDB()
	if err != nil {
		return err
	}
	if p.ID == 0 || key == "" {
		return errors.New("ymdb: saved post and metadata key are required")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var pm PostMeta
		err := tx.Where("post_id = ? AND key = ?", p.ID, key).Order("id").First(&pm).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&PostMeta{PostID: p.ID, Key: key, Value: value, Type: normalizeMetaType(valueType)}).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&pm).Updates(map[string]any{"value": value, "type": normalizeMetaType(valueType)}).Error
	})
}

func (p *Post) NewMeta(key, value, valueType string) { _, _ = p.AddMeta(key, value, valueType) }
func (p *Post) NewStringMeta(key, value string)      { p.NewMeta(key, value, MetaTypeString) }
func (p *Post) NewFloatMeta(key string, value float32) {
	p.NewMeta(key, strconv.FormatFloat(float64(value), 'g', -1, 32), MetaTypeFloat)
}
func (p *Post) NewIntMeta(key string, value int) { p.NewMeta(key, strconv.Itoa(value), MetaTypeInt) }
func (p *Post) NewBoolMeta(key string, value bool) {
	p.NewMeta(key, strconv.FormatBool(value), MetaTypeBool)
}
func (p *Post) NewJSONMeta(key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = p.AddMeta(key, string(b), MetaTypeJSON)
	return err
}

func (p *Post) FindMeta(key string) (Meta, error) {
	db, err := defaultDB()
	if err != nil {
		return Meta{}, err
	}
	var pm PostMeta
	if err := db.Where("post_id = ? AND key = ?", p.ID, key).Order("id").First(&pm).Error; err != nil {
		return Meta{}, err
	}
	return Meta{Value: pm.Value, Type: normalizeMetaType(pm.Type)}, nil
}
func (p *Post) GetMeta(key string) Meta { m, _ := p.FindMeta(key); return m }

func (p *Post) MetaMap() (map[string]Meta, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []PostMeta
	if err := db.Where("post_id = ?", p.ID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]Meta, len(rows))
	for _, row := range rows {
		result[row.Key] = Meta{Value: row.Value, Type: normalizeMetaType(row.Type)}
	}
	return result, nil
}
func (p *Post) GetMetaMap() map[string]Meta { m, _ := p.MetaMap(); return m }

// MetaValueMap preserves every metadata row, including repeated keys.
func (p *Post) MetaValueMap() (map[string][]Meta, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []PostMeta
	if err := db.Where("post_id = ?", p.ID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string][]Meta)
	for _, row := range rows {
		result[row.Key] = append(result[row.Key], Meta{Value: row.Value, Type: normalizeMetaType(row.Type)})
	}
	return result, nil
}

func (p *Post) GetMetaSlice(key string) []string {
	db, err := defaultDB()
	if err != nil {
		return nil
	}
	var rows []PostMeta
	if db.Where("post_id = ? AND key = ?", p.ID, key).Order("id").Find(&rows).Error != nil {
		return nil
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.Value)
	}
	return values
}
func (p *Post) UpdateMeta(key, value string) {
	m, err := p.FindMeta(key)
	if err == nil {
		_ = p.SetMeta(key, value, m.Type)
	}
}
func (p *Post) UpdateMetaWithType(key, value, valueType string) { _ = p.SetMeta(key, value, valueType) }
func (p *Post) DeleteMetaE(key string) error {
	db, err := defaultDB()
	if err != nil {
		return err
	}
	return db.Where("post_id = ? AND key = ?", p.ID, key).Delete(&PostMeta{}).Error
}
func (p *Post) DeleteMeta(key string) { _ = p.DeleteMetaE(key) }
