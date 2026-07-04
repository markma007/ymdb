package ymdb

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ParentPost returns the immediate parent. Root posts return gorm.ErrRecordNotFound.
func (p *Post) ParentPost() (*Post, error) {
	if p.ParentID == nil {
		return nil, gorm.ErrRecordNotFound
	}
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var parent Post
	if err := activePosts(db).First(&parent, *p.ParentID).Error; err != nil {
		return nil, err
	}
	return &parent, nil
}

// ChildPosts returns immediate children in stable tree order.
func (p *Post) ChildPosts() ([]Post, error) {
	if p.ID == 0 {
		return nil, errors.New("post must be saved")
	}
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var children []Post
	err = activePosts(db).Where("parent_id = ?", p.ID).Order("position, id").Find(&children).Error
	return children, err
}

func RootPosts(postType string) ([]Post, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	query := activePosts(db).Where("parent_id IS NULL")
	if postType != "" {
		query = query.Where("post_type = ?", postType)
	}
	var roots []Post
	err = query.Order("position, id").Find(&roots).Error
	return roots, err
}

func (p *Post) Ancestors() ([]Post, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	ancestors := []Post{}
	seen := map[uint]bool{p.ID: true}
	parentID := p.ParentID
	for parentID != nil {
		if seen[*parentID] {
			return nil, errors.New("ymdb: cycle detected in post hierarchy")
		}
		seen[*parentID] = true
		var parent Post
		if err := activePosts(db).First(&parent, *parentID).Error; err != nil {
			return nil, err
		}
		ancestors = append(ancestors, parent)
		parentID = parent.ParentID
	}
	return ancestors, nil
}

// Descendants returns breadth-first descendants, with sibling order preserved.
func (p *Post) Descendants() ([]Post, error) {
	if p.ID == 0 {
		return nil, errors.New("post must be saved")
	}
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	result := []Post{}
	level := []uint{p.ID}
	seen := map[uint]bool{p.ID: true}
	for len(level) > 0 {
		var rows []Post
		if err := activePosts(db).Where("parent_id IN ?", level).Order("position, id").Find(&rows).Error; err != nil {
			return nil, err
		}
		level = level[:0]
		for _, row := range rows {
			if seen[row.ID] {
				return nil, errors.New("ymdb: cycle detected in post hierarchy")
			}
			seen[row.ID] = true
			result = append(result, row)
			level = append(level, row.ID)
		}
	}
	return result, nil
}

func (p *Post) Siblings() ([]Post, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	var siblings []Post
	query := activePosts(db).Where("id <> ?", p.ID)
	if p.ParentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *p.ParentID)
	}
	err = query.Order("position, id").Find(&siblings).Error
	return siblings, err
}

func (p *Post) PreviousSibling() (*Post, error) {
	return p.adjacentSibling("position < ?", "position DESC, id DESC")
}
func (p *Post) NextSibling() (*Post, error) { return p.adjacentSibling("position > ?", "position, id") }

func (p *Post) adjacentSibling(positionWhere, order string) (*Post, error) {
	db, err := defaultDB()
	if err != nil {
		return nil, err
	}
	query := activePosts(db).Where(positionWhere, p.Position)
	if p.ParentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *p.ParentID)
	}
	var sibling Post
	if err := query.Order(order).First(&sibling).Error; err != nil {
		return nil, err
	}
	return &sibling, nil
}

// MoveTo moves or creates p below parent. A nil parent means root. A negative
// position appends; otherwise siblings at and after the position are shifted.
func (p *Post) MoveTo(parent *Post, position int) error {
	if err := p.Validate(); err != nil {
		return err
	}
	db, err := defaultDB()
	if err != nil {
		return err
	}
	if parent != nil && parent.ID == 0 {
		return errors.New("parent post must be saved")
	}
	if parent != nil && p.ID != 0 {
		if parent.ID == p.ID {
			return errors.New("a post cannot be its own parent")
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var parentID *uint
		if parent != nil {
			id := parent.ID
			parentID = &id
		}
		if p.ID != 0 && parentID != nil {
			cycle, err := postIsAncestor(tx, p.ID, *parentID)
			if err != nil {
				return err
			}
			if cycle {
				return errors.New("moving post would create a cycle")
			}
		}
		if p.ID != 0 {
			old := activePosts(tx.Model(&Post{})).Where("position > ? AND id <> ?", p.Position, p.ID)
			if p.ParentID == nil {
				old = old.Where("parent_id IS NULL")
			} else {
				old = old.Where("parent_id = ?", *p.ParentID)
			}
			if err := old.UpdateColumn("position", gorm.Expr("position - 1")).Error; err != nil {
				return err
			}
		}
		var count int64
		q := activePosts(tx.Model(&Post{}))
		if p.ID != 0 {
			q = q.Where("id <> ?", p.ID)
		}
		if parentID == nil {
			q = q.Where("parent_id IS NULL")
		} else {
			q = q.Where("parent_id = ?", *parentID)
		}
		if err := q.Count(&count).Error; err != nil {
			return err
		}
		if position < 0 || position > int(count) {
			position = int(count)
		}
		shift := activePosts(tx.Model(&Post{})).Where("position >= ?", position)
		if parentID == nil {
			shift = shift.Where("parent_id IS NULL")
		} else {
			shift = shift.Where("parent_id = ?", *parentID)
		}
		if p.ID != 0 {
			shift = shift.Where("id <> ?", p.ID)
		}
		if err := shift.UpdateColumn("position", gorm.Expr("position + 1")).Error; err != nil {
			return err
		}
		p.ParentID, p.Position = parentID, position
		if p.ID == 0 {
			return tx.Create(p).Error
		}
		return tx.Model(p).Updates(map[string]any{"parent_id": parentID, "position": position}).Error
	})
}

func postIsAncestor(tx *gorm.DB, candidateID, nodeID uint) (bool, error) {
	seen := map[uint]bool{}
	for {
		if nodeID == candidateID {
			return true, nil
		}
		if seen[nodeID] {
			return false, errors.New("ymdb: existing cycle detected in post hierarchy")
		}
		seen[nodeID] = true
		var node Post
		if err := activePosts(tx.Select("id", "parent_id")).First(&node, nodeID).Error; err != nil {
			return false, err
		}
		if node.ParentID == nil {
			return false, nil
		}
		nodeID = *node.ParentID
	}
}

func (p *Post) Detach() error { return p.MoveTo(nil, -1) }

// DeleteSubtree soft-deletes the post, all descendants, and their metadata.
func (p *Post) DeleteSubtree() error {
	db, err := defaultDB()
	if err != nil {
		return err
	}
	if p.ID == 0 {
		return errors.New("post must be saved")
	}
	descendants, err := p.Descendants()
	if err != nil {
		return err
	}
	ids := make([]uint, 0, len(descendants)+1)
	ids = append(ids, p.ID)
	for _, node := range descendants {
		ids = append(ids, node.ID)
	}
	files, err := quarantineAttachmentFiles(db, append([]Post{*p}, descendants...))
	if err != nil {
		return err
	}
	rollbackFiles := true
	defer func() {
		if rollbackFiles {
			restoreQuarantinedFiles(files)
		}
	}()
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("post_id IN ?", ids).Delete(&PostMeta{}).Error; err != nil {
			return fmt.Errorf("delete subtree metadata: %w", err)
		}
		if err := tx.Model(&Post{}).Where("id IN ?", ids).Update("status", PostStatusDeleted).Error; err != nil {
			return fmt.Errorf("mark subtree deleted: %w", err)
		}
		if err := tx.Where("id IN ?", ids).Delete(&Post{}).Error; err != nil {
			return fmt.Errorf("delete subtree posts: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	rollbackFiles = false
	if err := removeQuarantinedFiles(files); err != nil {
		return err
	}
	p.Status = PostStatusDeleted
	return nil
}
