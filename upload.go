package ymdb

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const DefaultMaxUploadBytes int64 = 64 << 20 // 64 MiB

const (
	UploadOptionGroup = DefaultOptionGroup
	UploadRootOption  = "upload_root"
)

type UploadConfig struct {
	Root        string
	Filename    string
	ContentType string
	MaxBytes    int64
	Parent      *Post
	Now         time.Time
}

type UploadResult struct {
	Post         *Post
	AbsolutePath string
	RelativePath string
	Size         int64
	SHA256       string
}

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// Upload streams a file into Root/YYYY/MM/DD and records it as an attachment
// post. When Root is blank, app/upload_root is read from the options table.
func Upload(reader io.Reader, config UploadConfig) (*UploadResult, error) {
	if reader == nil {
		return nil, errors.New("ymdb: upload reader is required")
	}
	configuredRoot := strings.TrimSpace(config.Root)
	var err error
	if configuredRoot == "" {
		configuredRoot, err = ConfiguredUploadRoot()
		if err != nil {
			return nil, err
		}
	}
	root, err := filepath.Abs(configuredRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve upload root: %w", err)
	}
	filename := sanitizeUploadFilename(config.Filename)
	if filename == "" {
		return nil, errors.New("ymdb: upload filename is required")
	}
	maxBytes := config.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxUploadBytes
	}
	now := config.Now
	if now.IsZero() {
		now = time.Now()
	}
	directory := filepath.Join(root, now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}
	storedName, err := uniqueUploadFilename(filename)
	if err != nil {
		return nil, err
	}
	absolutePath := filepath.Join(directory, storedName)
	if !pathWithinRoot(root, absolutePath) {
		return nil, errors.New("ymdb: unsafe upload path")
	}

	temp, err := os.CreateTemp(directory, ".ymdb-upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary upload: %w", err)
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
			_ = os.Remove(absolutePath)
		}
	}()

	buffered := bufio.NewReader(reader)
	header, peekErr := buffered.Peek(512)
	if peekErr != nil && !errors.Is(peekErr, io.EOF) && !errors.Is(peekErr, bufio.ErrBufferFull) {
		return nil, fmt.Errorf("read upload header: %w", peekErr)
	}
	contentType := strings.TrimSpace(config.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(header)
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(buffered, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("write upload: %w", err)
	}
	if written > maxBytes {
		return nil, fmt.Errorf("ymdb: upload exceeds %d byte limit", maxBytes)
	}
	if err := temp.Sync(); err != nil {
		return nil, fmt.Errorf("sync upload: %w", err)
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("close upload: %w", err)
	}
	if err := os.Rename(tempPath, absolutePath); err != nil {
		return nil, fmt.Errorf("commit upload: %w", err)
	}
	if err := os.Chmod(absolutePath, 0o640); err != nil {
		return nil, fmt.Errorf("set upload permissions: %w", err)
	}

	relativePath, err := filepath.Rel(root, absolutePath)
	if err != nil {
		return nil, err
	}
	relativePath = filepath.ToSlash(relativePath)
	digest := hex.EncodeToString(hash.Sum(nil))
	attachment := &Post{PostType: "attachment", Title: config.Filename, Slug: strings.TrimSuffix(storedName, filepath.Ext(storedName)), Mime: PostMimeText, Data: relativePath, Format: PostFormatText, Revision: 1, Status: PostStatusDraft}
	if err := attachment.MoveTo(config.Parent, -1); err != nil {
		return nil, fmt.Errorf("create attachment post: %w", err)
	}
	if err := saveUploadMeta(attachment, root, storedName, contentType, written, digest); err != nil {
		db, _ := defaultDB()
		if db != nil {
			_ = db.Unscoped().Delete(attachment).Error
		}
		return nil, err
	}
	committed = true
	return &UploadResult{Post: attachment, AbsolutePath: absolutePath, RelativePath: relativePath, Size: written, SHA256: digest}, nil
}

// ConfiguredUploadRoot reads the global app/upload_root option.
func ConfiguredUploadRoot() (string, error) {
	option, err := OptionGet(UploadOptionGroup, UploadRootOption)
	if err != nil {
		return "", fmt.Errorf("ymdb: read %s/%s option: %w", UploadOptionGroup, UploadRootOption, err)
	}
	root := strings.TrimSpace(option.Value)
	if root == "" {
		return "", fmt.Errorf("ymdb: %s/%s option is empty", UploadOptionGroup, UploadRootOption)
	}
	name, err := OptionGet(DefaultOptionGroup, "name")
	if err == nil {
		root = strings.ReplaceAll(root, "{app_name}", sanitizePathToken(name.Value))
	}
	if root == "~" || strings.HasPrefix(root, "~"+string(filepath.Separator)) || strings.HasPrefix(root, "~/") || strings.HasPrefix(root, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("ymdb: resolve upload home: %w", err)
		}
		tail := strings.TrimLeft(root[1:], "/\\")
		root = filepath.Join(home, filepath.FromSlash(strings.ReplaceAll(tail, "\\", "/")))
	}
	return root, nil
}

func sanitizePathToken(value string) string {
	value = unsafeFilenameChars.ReplaceAllString(strings.TrimSpace(value), "-")
	value = strings.Trim(value, ".-")
	if value == "" {
		return "app"
	}
	return value
}

// SetUploadRoot stores the global upload directory as app/upload_root.
func SetUploadRoot(root string) (OptionModel, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return OptionModel{}, errors.New("ymdb: upload root is required")
	}
	return OptionSet(UploadOptionGroup, UploadRootOption, root, MetaTypeString)
}

func saveUploadMeta(post *Post, root, storedName, contentType string, size int64, digest string) error {
	db, err := defaultDB()
	if err != nil {
		return err
	}
	rows := []PostMeta{
		{PostID: post.ID, Key: "original_filename", Value: post.Title, Type: MetaTypeString},
		{PostID: post.ID, Key: "stored_filename", Value: storedName, Type: MetaTypeString},
		{PostID: post.ID, Key: "upload_root", Value: root, Type: MetaTypeString},
		{PostID: post.ID, Key: "file_mime", Value: contentType, Type: MetaTypeString},
		{PostID: post.ID, Key: "size_bytes", Value: fmt.Sprintf("%d", size), Type: MetaTypeInt},
		{PostID: post.ID, Key: "sha256", Value: digest, Type: MetaTypeString},
	}
	if err := db.Create(&rows).Error; err != nil {
		return fmt.Errorf("save upload metadata: %w", err)
	}
	return nil
}

// Upload attaches a streamed file beneath this post. Pass an empty uploadRoot
// to use the app/upload_root option.
func (p *Post) Upload(reader io.Reader, filename, uploadRoot string) (*UploadResult, error) {
	return Upload(reader, UploadConfig{Root: uploadRoot, Filename: filename, Parent: p})
}

// UploadFile copies an existing file into managed upload storage.
func UploadFile(sourcePath, uploadRoot string, parent *Post) (*UploadResult, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open upload source: %w", err)
	}
	defer file.Close()
	return Upload(file, UploadConfig{Root: uploadRoot, Filename: filepath.Base(sourcePath), Parent: parent})
}

func sanitizeUploadFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Trim(name, ". ")
	name = unsafeFilenameChars.ReplaceAllString(name, "-")
	if len(name) > 180 {
		ext := filepath.Ext(name)
		name = strings.TrimSuffix(name, ext)[:160] + ext
	}
	return name
}

func uniqueUploadFilename(name string) (string, error) {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate upload name: %w", err)
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return base + "-" + hex.EncodeToString(bytes) + ext, nil
}

func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
