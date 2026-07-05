package ymdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type resourceJSON struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type postJSON struct {
	resourceJSON
	PostType string `json:"post_type"`
	Title    string `json:"title"`
	Slug     string `json:"slug"`
	Content  string `json:"content"`
	Mime     string `json:"mime"`
	Data     string `json:"data"`
	Format   string `json:"format"`
	ParentID *uint  `json:"parent_id,omitempty"`
	Position int    `json:"position"`
	Revision int    `json:"revision"`
	Status   string `json:"status"`
	Meta     any    `json:"meta"`
}

type optionJSON struct {
	resourceJSON
	Group string `json:"group"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type userJSON struct {
	resourceJSON
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Meta     map[string]Meta `json:"meta"`
}

type deepUserJSON struct {
	User userJSON          `json:"user"`
	Meta map[string][]Meta `json:"meta"`
}

func resource(modelID uint, createdAt, updatedAt time.Time) resourceJSON {
	return resourceJSON{ID: modelID, CreatedAt: createdAt, UpdatedAt: updatedAt}
}

// JSONBytes returns a post and its metadata as an HTTP-ready JSON document.
func (p *Post) JSONBytes() ([]byte, error) {
	if p == nil {
		return nil, errors.New("ymdb: cannot encode a nil post")
	}
	meta, err := p.MetaMap()
	if err != nil {
		return nil, err
	}
	return json.Marshal(postJSON{
		resourceJSON: resource(p.ID, p.CreatedAt, p.UpdatedAt),
		PostType:     p.PostType, Title: p.Title, Slug: p.Slug, Content: p.Content,
		Mime: p.Mime, Data: p.Data, Format: p.Format, ParentID: p.ParentID,
		Position: p.Position, Revision: p.Revision, Status: p.Status, Meta: meta,
	})
}

func (p *Post) ToJSON() (string, error) { b, err := p.JSONBytes(); return string(b), err }
func (p *Post) ToJson() (string, error) { return p.ToJSON() }

// ToDeepJSON returns the post plus every associated metadata value. Metadata
// is grouped by key, but each key contains a slice because PostMeta supports
// repeated keys. This prevents deep serialization from discarding records.
func (p *Post) ToDeepJSON() (string, error) {
	if p == nil {
		return "", errors.New("ymdb: cannot encode a nil post")
	}
	meta, err := p.MetaValueMap()
	if err != nil {
		return "", err
	}
	if meta == nil {
		meta = map[string][]Meta{}
	}
	payload := postJSON{
		resourceJSON: resource(p.ID, p.CreatedAt, p.UpdatedAt),
		PostType:     p.PostType, Title: p.Title, Slug: p.Slug, Content: p.Content,
		Mime: p.Mime, Data: p.Data, Format: p.Format, ParentID: p.ParentID,
		Position: p.Position, Revision: p.Revision, Status: p.Status, Meta: meta,
	}
	b, err := json.Marshal(payload)
	return string(b), err
}

func (p *Post) ToDeepJson() (string, error) { return p.ToDeepJSON() }

// Dump pretty-prints the post and all of its metadata to standard output.
// It is intended for visual inspection during application development.
func (p *Post) Dump() error {
	js, err := p.ToDeepJSON()
	if err != nil {
		return err
	}
	return dumpJSON(js)
}

// ToDeepJsonString retains the original ID-based helper. Prefer the Post
// method when the instance is already loaded so errors can be handled.
func ToDeepJsonString(id int) string {
	db, err := defaultDB()
	if err != nil {
		return ""
	}
	var post Post
	if err := activePosts(db).First(&post, id).Error; err != nil {
		return ""
	}
	result, _ := post.ToDeepJSON()
	return result
}

// JSONBytes returns an option. Group is always present because it is part of
// the option's identity together with Key.
func (o *OptionModel) JSONBytes() ([]byte, error) {
	if o == nil {
		return nil, errors.New("ymdb: cannot encode a nil option")
	}
	return json.Marshal(optionJSON{
		resourceJSON: resource(o.ID, o.CreatedAt, o.UpdatedAt),
		Group:        o.Group, Key: o.Key, Value: o.Value,
	})
}

func (o *OptionModel) ToJSON() (string, error) { b, err := o.JSONBytes(); return string(b), err }
func (o *OptionModel) ToJson() (string, error) { return o.ToJSON() }

// Dump pretty-prints the option to standard output. It is intended for visual
// inspection during application development.
func (o *OptionModel) Dump() error {
	js, err := o.ToJSON()
	if err != nil {
		return err
	}
	return dumpJSON(js)
}

// JSONBytes deliberately excludes the user's password hash.
func (u *User) JSONBytes() ([]byte, error) {
	if u == nil {
		return nil, errors.New("ymdb: cannot encode a nil user")
	}
	meta, err := u.MetaMap()
	if err != nil {
		return nil, err
	}
	return json.Marshal(userJSON{
		resourceJSON: resource(u.ID, u.CreatedAt, u.UpdatedAt),
		Username:     u.Username, Email: u.Email, Meta: meta,
	})
}

func (u *User) ToJSON() (string, error) { b, err := u.JSONBytes(); return string(b), err }
func (u *User) ToJson() (string, error) { return u.ToJSON() }

// ToDeepJSON returns a user plus every metadata value. Password is excluded.
func (u *User) ToDeepJSON() (string, error) {
	if u == nil {
		return "", errors.New("ymdb: cannot encode a nil user")
	}
	meta, err := u.MetaValueMap()
	if err != nil {
		return "", err
	}
	payload := userJSON{
		resourceJSON: resource(u.ID, u.CreatedAt, u.UpdatedAt),
		Username:     u.Username, Email: u.Email,
	}
	b, err := json.Marshal(deepUserJSON{User: payload, Meta: meta})
	return string(b), err
}

func (u *User) ToDeepJson() (string, error) { return u.ToDeepJSON() }

// Dump pretty-prints the user and all of its metadata to standard output.
// The password hash is never included.
func (u *User) Dump() error {
	js, err := u.ToDeepJSON()
	if err != nil {
		return err
	}
	return dumpJSON(js)
}

func dumpJSON(js string) error {
	formatted, err := PrettyPrintJSON(js, 4)
	if err != nil {
		return err
	}
	if _, err := fmt.Println(formatted); err != nil {
		return fmt.Errorf("ymdb: print JSON: %w", err)
	}
	return nil
}

func UserToDeepJsonString(id int) string {
	db, err := defaultDB()
	if err != nil {
		return ""
	}
	var user User
	if err := db.First(&user, id).Error; err != nil {
		return ""
	}
	result, _ := user.ToDeepJSON()
	return result
}

// PostsToJSON serializes a collection with one metadata query, avoiding N+1
// queries when producing list responses.
func PostsToJSON(posts []Post) (string, error) {
	ids := make([]uint, len(posts))
	for i := range posts {
		ids[i] = posts[i].ID
	}
	meta, err := bulkPostMeta(ids)
	if err != nil {
		return "", err
	}
	payload := make([]postJSON, 0, len(posts))
	for i := range posts {
		p := &posts[i]
		postMeta := meta[p.ID]
		if postMeta == nil {
			postMeta = map[string]Meta{}
		}
		payload = append(payload, postJSON{resourceJSON: resource(p.ID, p.CreatedAt, p.UpdatedAt), PostType: p.PostType, Title: p.Title, Slug: p.Slug, Content: p.Content, Mime: p.Mime, Data: p.Data, Format: p.Format, ParentID: p.ParentID, Position: p.Position, Revision: p.Revision, Status: p.Status, Meta: postMeta})
	}
	b, err := json.Marshal(payload)
	return string(b), err
}

func OptionsToJSON(options []OptionModel) (string, error) {
	payload := make([]optionJSON, 0, len(options))
	for i := range options {
		o := &options[i]
		payload = append(payload, optionJSON{resourceJSON: resource(o.ID, o.CreatedAt, o.UpdatedAt), Group: o.Group, Key: o.Key, Value: o.Value})
	}
	b, err := json.Marshal(payload)
	return string(b), err
}

func UsersToJSON(users []User) (string, error) {
	ids := make([]uint, len(users))
	for i := range users {
		ids[i] = users[i].ID
	}
	meta, err := bulkUserMeta(ids)
	if err != nil {
		return "", err
	}
	payload := make([]userJSON, 0, len(users))
	for i := range users {
		u := &users[i]
		userMeta := meta[u.ID]
		if userMeta == nil {
			userMeta = map[string]Meta{}
		}
		payload = append(payload, userJSON{resourceJSON: resource(u.ID, u.CreatedAt, u.UpdatedAt), Username: u.Username, Email: u.Email, Meta: userMeta})
	}
	b, err := json.Marshal(payload)
	return string(b), err
}

func bulkPostMeta(ids []uint) (map[uint]map[string]Meta, error) {
	result := map[uint]map[string]Meta{}
	if len(ids) == 0 {
		return result, nil
	}
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []PostMeta
	if err := db.Where("post_id IN ?", ids).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if result[row.PostID] == nil {
			result[row.PostID] = map[string]Meta{}
		}
		result[row.PostID][row.Key] = Meta{Value: row.Value, Type: normalizeMetaType(row.Type)}
	}
	return result, nil
}

func bulkUserMeta(ids []uint) (map[uint]map[string]Meta, error) {
	result := map[uint]map[string]Meta{}
	if len(ids) == 0 {
		return result, nil
	}
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var rows []UserMeta
	if err := db.Where("user_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if result[row.UserID] == nil {
			result[row.UserID] = map[string]Meta{}
		}
		result[row.UserID][row.Key] = Meta{Value: row.Value, Type: normalizeMetaType(row.Type)}
	}
	return result, nil
}
