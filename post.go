package ymdb

import (
	"errors"

	"gorm.io/gorm"
)

const (
	PostStatusDraft   = "draft"
	PostStatusPublish = "publish"
	PostStatusDeleted = "deleted"
)

type Post struct {
	gorm.Model
	// Basic
	PostType string `json:"post_type"`
	Title    string `json:"title"`
	Slug     string `json:"slug"` // "post_type_id"
	Content  string `json:"content"`
	Mime     string `json:"mime"` // Representation of Content: text, html, markdown, etc.
	Data     string `json:"data"`
	Format   string `json:"format"` // Encoding of Data: text, json, csv, or tabular.
	// Relations
	ParentID *uint `json:"parent_id,omitempty" gorm:"index:idx_post_siblings,priority:1"`
	Parent   *Post `json:"-" gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Position int   `json:"position" gorm:"not null;default:0;index:idx_post_siblings,priority:2"`
	//
	Revision int    `json:"revision"`
	Status   string `json:"status" gorm:"not null;default:draft;index"`
	//
}

func NewPost(post_type string) *Post {
	p, _ := NewPostE(post_type)
	return p
}

func NewPostE(postType string) (*Post, error) {
	p := &Post{
		PostType: postType,
		//
		Title:   "",
		Slug:    "",
		Content: "",
		Mime:    PostMimeText,
		Data:    "",
		Format:  PostFormatText,
		//
		//
		Revision: 1,
		Status:   PostStatusDraft,
	}
	if err := p.MoveTo(nil, -1); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Post) SaveE() error {
	if err := p.Validate(); err != nil {
		return err
	}
	db, err := defaultDB()
	if err != nil {
		return err
	}
	return db.Save(p).Error
}
func (p *Post) Save() { _ = p.SaveE() }

// Publish marks a saved post as publicly visible.
func (p *Post) Publish() error {
	if p == nil || p.ID == 0 {
		return errors.New("ymdb: post must be saved before publishing")
	}
	if p.DeletedAt.Valid || p.Status == PostStatusDeleted || p.Status == "delete" {
		return errors.New("ymdb: deleted post cannot be published")
	}
	db, err := defaultDB()
	if err != nil {
		return err
	}
	result := db.Model(p).Update("status", PostStatusPublish)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("ymdb: post was not published")
	}
	p.Status = PostStatusPublish
	return nil
}

// Delete soft-deletes this post and its subtree, including attachment files.
func (p *Post) Delete() error {
	if p == nil || p.ID == 0 {
		return errors.New("ymdb: post must be saved before deletion")
	}
	if err := p.DeleteSubtree(); err != nil {
		return err
	}
	p.Status = PostStatusDeleted
	return nil
}

func activePosts(db *gorm.DB) *gorm.DB {
	return db.Where("status NOT IN ?", []string{PostStatusDeleted, "delete"})
}

func (p *Post) Update(field, value string) {
	db, err := defaultDB()
	if err == nil {
		_ = db.Model(p).Update(field, value).Error
	}
}

func (p *Post) AddChildSameType() (*Post, error) {
	return p.AddChildWithPostType(p.PostType)
}

func (p *Post) AddAttachment() (*Post, error) {
	return p.AddChildWithPostType("attachment")
}

func (p *Post) AddChildWithPostType(post_type string) (*Post, error) {
	if p.ID == 0 {
		return nil, errors.New("parent post must be saved")
	}
	c := &Post{PostType: post_type, Mime: PostMimeText, Format: PostFormatText, Revision: 1, Status: PostStatusDraft}
	if err := c.MoveTo(p, -1); err != nil {
		return nil, err
	}
	return c, nil
}

func (p *Post) AddNext() (*Post, error) {
	if p.ID == 0 {
		return nil, errors.New("post must be saved")
	}
	next := &Post{PostType: p.PostType, Mime: PostMimeText, Format: PostFormatText, Revision: 1, Status: PostStatusDraft}
	var parent *Post
	if p.ParentID != nil {
		found, err := p.ParentPost()
		if err != nil {
			return nil, err
		}
		parent = found
	}
	if err := next.MoveTo(parent, p.Position+1); err != nil {
		return nil, err
	}
	return next, nil
}

func (p *Post) AdoptChild(c *Post) {
	_ = p.AdoptChildE(c)
}

func (p *Post) AdoptChildE(c *Post) error {
	if p.ID == 0 {
		return errors.New("parent post must be saved")
	}
	if c == nil {
		return errors.New("child post is required")
	}
	return c.MoveTo(p, -1)
}

func GetPostByID(id int) Post {
	post, _ := FindPostByID(id)
	return post
}

func FindPostByID(id int) (Post, error) {
	var post Post
	db, err := defaultDB()
	if err != nil {
		return post, err
	}
	err = activePosts(db).First(&post, id).Error
	return post, err
}

func QueryPosts(query interface{}, args ...interface{}) []Post {
	posts, _ := QueryPostsE(query, args...)
	return posts
}

func QueryPostsE(query interface{}, args ...interface{}) ([]Post, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var posts []Post
	err = activePosts(db.Model(Post{})).Where(query, args...).Find(&posts).Error
	return posts, err
}

func QueryPagePosts(page_id, page_size int, query interface{}, args ...interface{}) []Post {
	var posts []Post
	db, err := defaultDB()
	if err != nil || page_id < 1 || page_size < 1 {
		return posts
	}
	if err := activePosts(db.Model(Post{})).
		Order("updated_at DESC").
		Limit(page_size).
		Offset(page_size*(page_id-1)).
		Where(query, args...).
		Find(&posts).Error; err != nil {
	}
	return posts
}

func PagePosts(page_id, page_size int, post_type string) []Post {
	var posts []Post
	db, err := defaultDB()
	if err != nil || page_id < 1 || page_size < 1 {
		return posts
	}
	if post_type != "" {
		if err := activePosts(db.Model(Post{})).
			Order("updated_at DESC").
			Limit(page_size).
			Offset((page_id-1)*page_size).
			Where("post_type=?", post_type).
			Find(&posts).Error; err != nil {
		}
	} else {
		if err := activePosts(db.Model(Post{})).
			Order("updated_at DESC").
			Limit(page_size).
			Offset((page_id - 1) * page_size).
			Find(&posts).Error; err != nil {
		}
	}
	return posts
}
