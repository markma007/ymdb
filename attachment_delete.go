package ymdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"
)

type quarantinedFile struct{ original, quarantine string }

func quarantineAttachmentFiles(db *gorm.DB, posts []Post) ([]quarantinedFile, error) {
	ids := make([]uint, 0)
	paths := map[uint]string{}
	for _, post := range posts {
		if post.PostType == "attachment" && post.Data != "" {
			ids = append(ids, post.ID)
			paths[post.ID] = post.Data
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []PostMeta
	if err := db.Where("post_id IN ? AND key = ?", ids, "upload_root").Find(&rows).Error; err != nil {
		return nil, err
	}
	roots := map[uint]string{}
	for _, row := range rows {
		roots[row.PostID] = row.Value
	}
	configuredRoot := ""
	files := make([]quarantinedFile, 0, len(ids))
	for _, id := range ids {
		root := roots[id]
		if root == "" {
			if configuredRoot == "" {
				var err error
				configuredRoot, err = ConfiguredUploadRoot()
				if err != nil {
					restoreQuarantinedFiles(files)
					return nil, fmt.Errorf("resolve legacy attachment root: %w", err)
				}
			}
			root = configuredRoot
		}
		root, err := filepath.Abs(root)
		if err != nil {
			restoreQuarantinedFiles(files)
			return nil, err
		}
		original := filepath.Join(root, filepath.FromSlash(paths[id]))
		if !pathWithinRoot(root, original) {
			restoreQuarantinedFiles(files)
			return nil, errors.New("ymdb: attachment path escapes upload root")
		}
		if _, err := os.Stat(original); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			restoreQuarantinedFiles(files)
			return nil, err
		}
		suffix, err := uniqueUploadFilename("delete")
		if err != nil {
			restoreQuarantinedFiles(files)
			return nil, err
		}
		quarantine := original + ".ymdb-" + suffix
		if err := os.Rename(original, quarantine); err != nil {
			restoreQuarantinedFiles(files)
			return nil, fmt.Errorf("quarantine attachment: %w", err)
		}
		files = append(files, quarantinedFile{original: original, quarantine: quarantine})
	}
	return files, nil
}

func restoreQuarantinedFiles(files []quarantinedFile) {
	for i := len(files) - 1; i >= 0; i-- {
		_ = os.Rename(files[i].quarantine, files[i].original)
	}
}

func removeQuarantinedFiles(files []quarantinedFile) error {
	var first error
	for _, file := range files {
		if err := os.Remove(file.quarantine); err != nil && !errors.Is(err, os.ErrNotExist) && first == nil {
			first = err
		}
	}
	return first
}
