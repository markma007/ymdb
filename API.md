# YMDB Library API

YMDB is a small, WordPress-inspired data library for Go. It provides posts, typed post and user metadata, grouped application options, users, tree navigation, JSON response helpers, fixtures, and filesystem-backed attachment uploads on SQLite.

## Contents

1. [Installation](#installation)
2. [Database lifecycle](#database-lifecycle)
3. [Default fixtures](#default-fixtures)
4. [Typed metadata](#typed-metadata)
5. [Posts](#posts)
6. [Post trees](#post-trees)
7. [Grouped options](#grouped-options)
8. [Users](#users)
9. [Attachments and uploads](#attachments-and-uploads)
10. [JSON responses](#json-responses)
11. [Error handling](#error-handling)
12. [Operational notes](#operational-notes)

## Installation

The current module path is:

```go
import "ym.com/ymdb"
```

If YMDB is kept beside another local project, add a replacement to that project's `go.mod`:

```go
require ym.com v0.0.0

replace ym.com => ../ym.com
```

Then run:

```shell
go mod tidy
```

If the repository is later published under a real Git host, update the module path and use `go get` normally.

## Database lifecycle

### Simple initialization

Use `InitiateDBM` when the application uses YMDB's package-level APIs:

```go
if err := ymdb.InitiateDBM("./data/application.sqlite"); err != nil {
    log.Fatal(err)
}
defer ymdb.DBM.Close()
```

Initialization performs these operations:

- Opens or creates the SQLite file.
- Enables foreign keys.
- Enables WAL journal mode.
- Sets a 5-second SQLite busy timeout.
- Runs schema migration.
- Installs missing default fixtures.

### Explicit manager

`Open` creates a manager without activating package-level model methods:

```go
manager, err := ymdb.Open("./data/application.sqlite")
if err != nil {
    return err
}
defer manager.Close()

if err := manager.Activate(); err != nil {
    return err
}
```

### Reconnect and close

```go
err := manager.Reconnect("./data/another.sqlite")
err := manager.Close()
```

`Reconnect` closes the previous connection pool after the new database opens successfully. `Close` also clears the package-global database when that manager is active.

## Default fixtures

YMDB embeds `ymdb/fixtures/default.json` and installs missing options during database initialization:

```json
{
  "options": [
    {"group": "app", "key": "name", "value": "my_app"},
    {"group": "app", "key": "title", "value": "My App"},
    {"group": "app", "key": "upload_root", "value": "~/{app_name}_uploads"}
  ]
}
```

Fixtures never replace an existing `(group, key)` option.

Install a custom fixture stream:

```go
file, err := os.Open("fixtures/production.json")
if err != nil {
    return err
}
defer file.Close()

if err := ymdb.InstallFixtures(ymdb.DB, file); err != nil {
    return err
}
```

Fixture format:

```json
{
  "options": [
    {
      "group": "plugin.search",
      "key": "enabled",
      "value": "true"
    }
  ]
}
```

## Typed metadata

Supported type constants:

```go
ymdb.MetaTypeString
ymdb.MetaTypeInt
ymdb.MetaTypeFloat
ymdb.MetaTypeBool
ymdb.MetaTypeJSON
```

Metadata is stored as a string value plus a type label:

```go
type Meta struct {
    Value string `json:"value"`
    Type  string `json:"type"`
}
```

Decode a value into its Go type:

```go
meta, err := post.FindMeta("views")
if err != nil {
    return err
}

var views int
if err := meta.Decode(&views); err != nil {
    return err
}
```

Expected destinations are `*string`, `*int`, `*float64`, `*bool`, or a JSON-compatible pointer for JSON metadata.

## Posts

### Model

Important post fields include:

```go
type Post struct {
    gorm.Model

    PostType string
    Title    string
    Slug     string
    Content  string
    Mime     string
    Data     string
    Format   string

    ParentID *uint
    Position int

    Revision int
    Status   string
}
```

`Mime` describes the representation of `Content`. Built-in constants are:

```go
ymdb.PostMimeText
ymdb.PostMimeHTML
ymdb.PostMimeMarkdown
```

Other non-empty content MIME labels may also be used.

`Format` describes the encoding of `Data` and must be one of:

```go
ymdb.PostFormatText
ymdb.PostFormatJSON
ymdb.PostFormatCSV
```

Use the encoding helpers instead of manually coordinating `Data` and `Format`:

```go
post.SetTextData("plain value")
err := post.SetJSONData(map[string]any{"enabled": true})
err = post.SetCSVData([][]string{{"name", "score"}, {"Alex", "10"}})

var document map[string]any
err = post.DecodeJSONData(&document)

records, err := post.DecodeCSVData()
```

Set content and its representation together:

```go
err := post.SetContent("# Heading", ymdb.PostMimeMarkdown)
```

### Create and save

Prefer the error-returning constructor:

```go
post, err := ymdb.NewPostE("article")
if err != nil {
    return err
}

post.Title = "Hello"
post.Slug = "hello"
post.Content = "Hello from YMDB"

if err := post.SaveE(); err != nil {
    return err
}
```

New posts start with `Status == ymdb.PostStatusDraft`. Publish explicitly:

```go
if err := post.Publish(); err != nil {
    return err
}
```

Lifecycle constants are `PostStatusDraft`, `PostStatusPublish`, and `PostStatusDeleted`.

Soft-delete a post and its subtree:

```go
if err := post.Delete(); err != nil {
    return err
}
```

Library post and tree queries exclude deleted posts. The legacy status value `"delete"` is also excluded.

`NewPost` and `Save` are compatibility helpers that discard errors. Avoid them in request handlers and production services.

### Retrieve and query

```go
post, err := ymdb.FindPostByID(42)

posts, err := ymdb.QueryPostsE(
    "post_type = ? AND status = ?",
    "article",
    "publish",
)

page := ymdb.PagePosts(1, 20, "article")
```

Page numbers begin at `1`. `PagePosts` orders results by `updated_at DESC`.

### Post metadata

`SetMeta` treats a key as a single current value:

```go
err := post.SetMeta("views", "10", ymdb.MetaTypeInt)
```

`AddMeta` permits multiple rows with the same key:

```go
_, err := post.AddMeta("tag", "go", ymdb.MetaTypeString)
_, err = post.AddMeta("tag", "sqlite", ymdb.MetaTypeString)
```

Convenience creation methods:

```go
post.NewStringMeta("author", "Alex")
post.NewIntMeta("views", 10)
post.NewFloatMeta("rating", 4.5)
post.NewBoolMeta("featured", true)
err := post.NewJSONMeta("layout", map[string]any{"columns": 3})
```

Read or delete metadata:

```go
meta, err := post.FindMeta("views")
values := post.GetMetaSlice("tag")
singleValueMap, err := post.MetaMap()
allValuesMap, err := post.MetaValueMap()
err = post.DeleteMetaE("views")
```

`MetaMap` keeps one value per key. Use `MetaValueMap` when repeated keys must be preserved.

## Post trees

Posts use an adjacency-list hierarchy. `ParentID` is the source of truth and `Position` controls sibling order.

### Create children and siblings

```go
root, err := ymdb.NewPostE("page")
child, err := root.AddChildSameType()
attachmentNode, err := root.AddAttachment()
nextSibling, err := child.AddNext()
```

Create a child with another post type:

```go
comment, err := root.AddChildWithPostType("comment")
```

### Navigate

```go
parent, err := child.ParentPost()
children, err := root.ChildPosts()
roots, err := ymdb.RootPosts("page")
ancestors, err := child.Ancestors()
descendants, err := root.Descendants()
siblings, err := child.Siblings()
previous, err := child.PreviousSibling()
next, err := child.NextSibling()
```

`ParentPost`, `PreviousSibling`, and `NextSibling` return `gorm.ErrRecordNotFound` when no matching node exists.

### Move and detach

```go
// Position -1 appends after existing children.
err := child.MoveTo(newParent, -1)

// Insert as the first child.
err = child.MoveTo(newParent, 0)

// Move to the root list.
err = child.Detach()
```

Moves are transactional, maintain sibling positions, and reject cycles and self-parenting.

### Delete a subtree

```go
err := root.DeleteSubtree()
```

This soft-deletes the post subtree and its post metadata. Attachment files are quarantined before the database transaction, restored if the transaction fails, and removed after a successful commit.

## Grouped options

An option is uniquely identified by `(group, key)`. A blank group becomes `app`.

### Set and get

```go
option, err := ymdb.OptionSet(
    "app",
    "title",
    "My Application",
)

option, err = ymdb.OptionGet("app", "title")
```

The same key can exist independently in different groups:

```go
_, _ = ymdb.OptionSet("app", "enabled", "true")
_, _ = ymdb.OptionSet("plugin.search", "enabled", "false")
```

### Read groups

```go
groups, err := ymdb.OptionGroupNamesE()
options, err := ymdb.OptionQueryByGroupE("plugin.search")
config, err := ymdb.OptionMap("plugin.search")
```

Option values are stored as strings. Applications are responsible for parsing
values such as booleans or numbers.

### Delete

```go
err := ymdb.OptionDelete("plugin.search", "enabled")
err = ymdb.OptionDeleteGroup("plugin.search")
```

## Users

YMDB accepts a password hash; it does not hash passwords itself.

```go
hash := hashPasswordInYourApplication(password)

user, err := ymdb.CreateUser(
    "alex",
    "alex@example.com",
    hash,
)
```

Usernames and emails are unique. Emails are trimmed and converted to lowercase.

User metadata:

```go
err := user.SetMeta("timezone", "America/Chicago", ymdb.MetaTypeString)
meta, err := user.MetaMap()
allMeta, err := user.MetaValueMap()
```

Password hashes are excluded from YMDB JSON serializers.

## Attachments and uploads

Uploads are stored under:

```text
UPLOAD_ROOT/YYYY/MM/DD/sanitized-random-name.ext
```

The default `app/upload_root` option is:

```text
~/{app_name}_uploads
```

`~` expands to the current user's home directory and `{app_name}` expands from `app/name`.

### Configure upload storage

```go
_, err := ymdb.SetUploadRoot("/srv/my-app/uploads")

root, err := ymdb.ConfiguredUploadRoot()
```

### Upload an HTTP multipart file

```go
file, header, err := request.FormFile("file")
if err != nil {
    return err
}
defer file.Close()

result, err := parentPost.Upload(file, header.Filename, "")
if err != nil {
    return err
}
```

Passing an empty upload root uses `app/upload_root`.

### Advanced upload

```go
result, err := ymdb.Upload(file, ymdb.UploadConfig{
    Root:        "/srv/my-app/uploads", // blank uses app/upload_root
    Filename:    header.Filename,
    ContentType: header.Header.Get("Content-Type"),
    MaxBytes:    10 << 20,
    Parent:      parentPost,
})
```

The default size limit is `ymdb.DefaultMaxUploadBytes`, currently 64 MiB.

Copy an existing file:

```go
result, err := ymdb.UploadFile(
    "./incoming/report.pdf",
    "", // use configured root
    parentPost,
)
```

### Upload result

```go
type UploadResult struct {
    Post         *Post
    AbsolutePath string
    RelativePath string
    Size         int64
    SHA256       string
}
```

The attachment post has `PostType == "attachment"`. Its portable relative path is text stored in `Data`, so `Format == PostFormatText`. The detected file media type is stored in the `file_mime` post metadata key; `Mime` remains the representation of the post's `Content`.

## JSON responses

### Single resources

```go
text, err := post.ToJSON()
bytes, err := post.JSONBytes()

text, err = option.ToJSON()
text, err = user.ToJSON()
```

Post and user JSON include a one-value-per-key `meta` object. Option JSON contains
only the option resource fields.

### Deep resources

Deep JSON preserves all post metadata values, including repeated keys:

```go
text, err := post.ToDeepJSON()
text, err = user.ToDeepJSON()
```

Example shape:

```json
{
  "post": {
    "id": 1,
    "post_type": "article",
    "title": "Hello"
  },
  "meta": {
    "tag": [
      {"value": "go", "type": "string"},
      {"value": "sqlite", "type": "string"}
    ]
  }
}
```

### Collections

Use the bulk serializers for HTTP list responses. Post and user serializers load
metadata in one query and avoid N+1 behavior:

```go
posts, err := ymdb.QueryPostsE("post_type = ?", "article")
if err != nil {
    return err
}

body, err := ymdb.PostsToJSON(posts)
body, err = ymdb.OptionsToJSON(options)
body, err = ymdb.UsersToJSON(users)
```

### HTTP response example

```go
func listArticles(w http.ResponseWriter, r *http.Request) {
    posts, err := ymdb.QueryPostsE(
        "post_type = ? AND status = ?",
        "article",
        "publish",
    )
    if err != nil {
        http.Error(w, "database error", http.StatusInternalServerError)
        return
    }

    body, err := ymdb.PostsToJSON(posts)
    if err != nil {
        http.Error(w, "encoding error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    _, _ = io.WriteString(w, body)
}
```

## Error handling

Prefer APIs that return errors:

- `NewPostE`
- `SaveE`
- `FindPostByID`
- `QueryPostsE`
- `OptionGroupNamesE`
- `OptionQueryByGroupE`
- `DeleteMetaE`
- `ToJSON` and `ToDeepJSON`

Compatibility helpers such as `NewPost`, `Save`, `GetPostByID`, and `OptionGetByID` suppress errors and should not be used where failures must be reported accurately.

Check missing records with GORM's sentinel error:

```go
post, err := ymdb.FindPostByID(id)
if errors.Is(err, gorm.ErrRecordNotFound) {
    // Return HTTP 404.
}
```

## Operational notes

- YMDB is designed for a single application process using a local SQLite file.
- SQLite writes are serialized through one connection for integrity.
- Do not place the SQLite database on an unsupported network filesystem.
- Back up both the SQLite database and the configured upload root.
- Use application-level password hashing such as Argon2id or bcrypt before calling `CreateUser`.
- Apply authentication and authorization in the host application.
- Validate or restrict permitted upload types before serving uploaded content publicly.
- Avoid passing untrusted request text as a raw SQL query string. Use placeholders and arguments.
- Call `Close` during graceful application shutdown.

## Minimal complete example

```go
package main

import (
    "fmt"
    "log"

    "ym.com/ymdb"
)

func main() {
    if err := ymdb.InitiateDBM("./application.sqlite"); err != nil {
        log.Fatal(err)
    }
    defer ymdb.DBM.Close()

    post, err := ymdb.NewPostE("article")
    if err != nil {
        log.Fatal(err)
    }

    post.Title = "First article"
    post.Content = "Hello from YMDB"
    if err := post.SaveE(); err != nil {
        log.Fatal(err)
    }

    if err := post.SetMeta("views", "0", ymdb.MetaTypeInt); err != nil {
        log.Fatal(err)
    }

    body, err := post.ToDeepJSON()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(body)
}
```
