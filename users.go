package ymdb

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type User struct {
	gorm.Model
	Username string `json:"username" gorm:"not null;uniqueIndex"`
	Email    string `json:"email" gorm:"not null;uniqueIndex"`
	// Password must contain an application-generated password hash, never plaintext.
	Password string `json:"-" gorm:"not null"`
}

type UserMeta struct {
	gorm.Model
	UserID uint   `json:"user_id" gorm:"not null;uniqueIndex:idx_user_meta_lookup,priority:1"`
	User   *User  `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Key    string `json:"key" gorm:"not null;uniqueIndex:idx_user_meta_lookup,priority:2"`
	Value  string `json:"value" gorm:"type:text"`
	Type   string `json:"type" gorm:"not null;default:string"`
}

func CreateUser(username, email, passwordHash string) (*User, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(username) == "" || strings.TrimSpace(email) == "" || passwordHash == "" {
		return nil, errors.New("ymdb: username, email, and password hash are required")
	}
	u := &User{Username: username, Email: strings.ToLower(strings.TrimSpace(email)), Password: passwordHash}
	if err := db.Create(u).Error; err != nil {
		return nil, err
	}
	return u, nil
}

func (u *User) SetMeta(key, value, valueType string) error {
	db, err := defaultDB()
	if err != nil {
		return err
	}
	if u.ID == 0 || key == "" {
		return errors.New("ymdb: saved user and metadata key are required")
	}
	row := UserMeta{UserID: u.ID, Key: key, Value: value, Type: normalizeMetaType(valueType)}
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "key"}}, DoUpdates: clause.Assignments(map[string]any{"value": value, "type": row.Type, "deleted_at": nil, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})}).Create(&row).Error
}

func (u *User) MetaMap() (map[string]Meta, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []UserMeta
	if err = db.Where("user_id = ?", u.ID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]Meta{}
	for _, r := range rows {
		out[r.Key] = Meta{Value: r.Value, Type: normalizeMetaType(r.Type)}
	}
	return out, nil
}

// MetaValueMap preserves every user metadata row, including repeated keys.
func (u *User) MetaValueMap() (map[string][]Meta, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []UserMeta
	if err := db.Where("user_id = ?", u.ID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string][]Meta)
	for _, row := range rows {
		result[row.Key] = append(result[row.Key], Meta{Value: row.Value, Type: normalizeMetaType(row.Type)})
	}
	return result, nil
}
