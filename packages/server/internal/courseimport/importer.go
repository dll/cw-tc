// Package courseimport 把“课程资源包”导入平台：课程 + 讲次 + 多类型资源。
//
// 资源类型不同，落库方式也不同（数据字典 §3.4）：
//   - HTML / Markdown / 课件 / 代码 / 冻结输出 → 复制到 ContentDir，按相对路径私有读；
//   - HTML 目录会整目录复制（含 assets/），保证相对引用的图片 / CSS / JS 可访问；
//   - MP4 / 音频 → 配置 COS 时上传 COS，否则回落到 ContentDir（与在线播放逻辑一致）。
//
// 导入只创建平台课程草稿（status=building），不绕过审核上架。
package courseimport

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInput          = errors.New("课程导入参数不合法")
	ErrCourseExists   = errors.New("课程码已存在")
	ErrSourceNotFound = errors.New("资源文件不存在或超出导入目录")
	ErrUnsupported    = errors.New("不支持的资源类型")
)

var courseCodeRE = regexp.MustCompile(`^[A-Za-z0-9_-]{2,32}$`)

var validKinds = map[string]bool{
	"html": true, "markdown": true, "video": true, "audio": true,
	"code": true, "frozen_output": true, "slide": true,
}

type Manifest struct {
	CourseCode string    `json:"course_code"`
	Name       string    `json:"name"`
	Intro      string    `json:"intro"`
	Line       string    `json:"line"`
	MajorID    *int64    `json:"major_id"`
	PriceCents int64     `json:"price_cents"`
	TeacherID  *int64    `json:"teacher_id"`
	SourceRoot string    `json:"source_root"`
	Lectures   []Lecture `json:"lectures"`
}

type Lecture struct {
	No        int     `json:"no"`
	Title     string  `json:"title"`
	IsPreview bool    `json:"is_preview"`
	Assets    []Asset `json:"assets"`
}

type Asset struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Path  string `json:"path"`
	Sort  int    `json:"sort"`
}

type Result struct {
	CourseCode string   `json:"course_code"`
	Lectures   int      `json:"lectures"`
	Assets     int      `json:"assets"`
	Storage    []string `json:"storage_keys"`
}

// Uploader 只要求上传能力，便于测试注入；COS 未配置时传 nil。
type Uploader interface {
	Upload(ctx context.Context, key string, r io.Reader) error
}

type Importer struct {
	DB         *sql.DB
	ContentDir string
	ImportRoot string
	Uploader   Uploader
}

type plannedAsset struct {
	lectureNo  int
	kind       string
	title      string
	sort       int
	sourcePath string
	storageKey string
	isDir      bool
	size       int64
	sha256     string
}

// Import 导入课程。文件先落盘 / 上传，再在一个事务里写课程、讲次与资源。
func (im *Importer) Import(ctx context.Context, m Manifest) (Result, error) {
	if im == nil || im.DB == nil || strings.TrimSpace(im.ContentDir) == "" || strings.TrimSpace(im.ImportRoot) == "" {
		return Result{}, ErrInput
	}
	m.CourseCode = strings.TrimSpace(m.CourseCode)
	m.Name = strings.TrimSpace(m.Name)
	m.Line = strings.TrimSpace(m.Line)
	if !courseCodeRE.MatchString(m.CourseCode) {
		return Result{}, fmt.Errorf("%w：课程码需为 2~32 位字母 / 数字 / _ / -", ErrInput)
	}
	if m.Name == "" {
		return Result{}, fmt.Errorf("%w：课程名称必填", ErrInput)
	}
	if m.PriceCents < 0 {
		return Result{}, fmt.Errorf("%w：价格不能为负数", ErrInput)
	}
	if m.TeacherID == nil || *m.TeacherID <= 0 { // platform 课程必须有归属教师（DB CHECK）
		return Result{}, fmt.Errorf("%w：请选择归属教师", ErrInput)
	}
	if m.Line == "" {
		m.Line = "main"
	}
	if m.Line != "main" && m.Line != "aux" {
		return Result{}, fmt.Errorf("%w：line 只能是 main 或 aux", ErrInput)
	}
	if len(m.Lectures) == 0 || len(m.Lectures) > 200 {
		return Result{}, fmt.Errorf("%w：讲次数量需为 1~200", ErrInput)
	}
	var exists int
	if err := im.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM course WHERE course_code = $1`, m.CourseCode).Scan(&exists); err != nil {
		return Result{}, err
	}
	if exists > 0 {
		return Result{}, ErrCourseExists
	}
	var teacherRole string
	if err := im.DB.QueryRowContext(ctx, `SELECT role FROM staff WHERE id = $1`, *m.TeacherID).Scan(&teacherRole); err != nil || teacherRole != "teacher" {
		return Result{}, fmt.Errorf("%w：归属教师不存在或角色不是教师", ErrInput)
	}
	if m.MajorID != nil {
		var majorCount int
		if err := im.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM major WHERE id = $1`, *m.MajorID).Scan(&majorCount); err != nil || majorCount == 0 {
			return Result{}, fmt.Errorf("%w：专业分类不存在", ErrInput)
		}
	}
	courseRoot, err := im.resolveSource(m.SourceRoot, m.CourseCode)
	if err != nil {
		return Result{}, err
	}
	plan, err := im.planAssets(courseRoot, m)
	if err != nil {
		return Result{}, err
	}

	localCleanup := make([]string, 0)
	for idx := range plan {
		item := &plan[idx]
		if item.kind == "video" || item.kind == "audio" {
			if im.Uploader != nil {
				if err := im.uploadAsset(ctx, item); err != nil {
					cleanupLocal(localCleanup)
					return Result{}, err
				}
				continue
			}
		}
		destination := filepath.Join(im.ContentDir, filepath.FromSlash(item.storageKey))
		if item.isDir {
			if err := copyTree(item.sourcePath, filepath.Dir(destination)); err != nil {
				cleanupLocal(localCleanup)
				return Result{}, err
			}
			localCleanup = append(localCleanup, filepath.Dir(destination))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			cleanupLocal(localCleanup)
			return Result{}, err
		}
		size, digest, err := copyWithHash(item.sourcePath, destination)
		if err != nil {
			cleanupLocal(localCleanup)
			return Result{}, err
		}
		item.size, item.sha256 = size, digest
		localCleanup = append(localCleanup, destination)
		// HTML 单文件：把同级 assets/ 一起复制，保证相对引用可用。
		if item.kind == "html" && !item.isDir {
			assetsDir := filepath.Join(filepath.Dir(item.sourcePath), "assets")
			if info, statErr := os.Stat(assetsDir); statErr == nil && info.IsDir() {
				target := filepath.Join(filepath.Dir(destination), "assets")
				if err := copyTree(assetsDir, target); err != nil {
					cleanupLocal(localCleanup)
					return Result{}, err
				}
				localCleanup = append(localCleanup, target)
			}
		}
	}

	tx, err := im.DB.BeginTx(ctx, nil)
	if err != nil {
		cleanupLocal(localCleanup)
		return Result{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	var courseID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO course(course_code, source, name, intro, line, major_id, status, teacher_id, price_cents,
		                   lecture_count, created_at, updated_at)
		 VALUES($1,'platform',$2,$3,$4,$5,'building',$6,$7,$8,$9,$9) RETURNING id`,
		m.CourseCode, m.Name, m.Intro, m.Line, m.MajorID, m.TeacherID, m.PriceCents,
		len(m.Lectures), now).Scan(&courseID); err != nil {
		cleanupLocal(localCleanup)
		return Result{}, err
	}
	assetCount := 0
	for _, lecture := range m.Lectures {
		var lectureID int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO lecture(course_id, no, title, is_preview, origin) VALUES($1,$2,$3,$4,'platform') RETURNING id`,
			courseID, lecture.No, strings.TrimSpace(lecture.Title), lecture.IsPreview).Scan(&lectureID); err != nil {
			cleanupLocal(localCleanup)
			return Result{}, err
		}
		for _, item := range plan {
			if item.lectureNo != lecture.No {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO asset(lecture_id, kind, title, storage_key, size_bytes, sha256, sort)
				 VALUES($1,$2,$3,$4,$5,$6,$7)`,
				lectureID, item.kind, item.title, item.storageKey, item.size, item.sha256, item.sort); err != nil {
				cleanupLocal(localCleanup)
				return Result{}, err
			}
			assetCount++
		}
	}
	if err := tx.Commit(); err != nil {
		cleanupLocal(localCleanup)
		return Result{}, err
	}
	keys := make([]string, 0, len(plan))
	for _, item := range plan {
		keys = append(keys, item.storageKey)
	}
	return Result{CourseCode: m.CourseCode, Lectures: len(m.Lectures), Assets: assetCount, Storage: keys}, nil
}

func (im *Importer) resolveSource(sourceRoot, fallback string) (string, error) {
	root, err := filepath.Abs(im.ImportRoot)
	if err != nil {
		return "", err
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("%w：资源包目录不可用", ErrSourceNotFound)
	}
	relative := strings.TrimSpace(sourceRoot)
	if relative == "" {
		relative = fallback
	}
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("%w：source_root 必须是相对路径", ErrSourceNotFound)
	}
	candidate := filepath.Join(rootResolved, filepath.FromSlash(relative))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("%w：%s", ErrSourceNotFound, relative)
	}
	if resolved != rootResolved && !strings.HasPrefix(resolved, rootResolved+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w：%s 超出资源包目录", ErrSourceNotFound, relative)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w：%s 不是目录", ErrSourceNotFound, relative)
	}
	return resolved, nil
}

func (im *Importer) planAssets(courseRoot string, m Manifest) ([]plannedAsset, error) {
	plan := make([]plannedAsset, 0)
	seenLecture := map[int]bool{}
	seenKey := map[string]bool{}
	for _, lecture := range m.Lectures {
		if lecture.No < 1 || strings.TrimSpace(lecture.Title) == "" || seenLecture[lecture.No] {
			return nil, fmt.Errorf("%w：第 %d 讲的讲次号或标题非法（讲次号必须为正且不重复）", ErrInput, lecture.No)
		}
		seenLecture[lecture.No] = true
		for _, asset := range lecture.Assets {
			rel := strings.TrimSpace(asset.Path)
			if rel == "" || filepath.IsAbs(rel) {
				return nil, fmt.Errorf("%w：第 %d 讲的资源路径非法 %q", ErrInput, lecture.No, asset.Path)
			}
			source := filepath.Join(courseRoot, filepath.FromSlash(rel))
			resolved, err := filepath.EvalSymlinks(source)
			if err != nil || (resolved != courseRoot && !strings.HasPrefix(resolved, courseRoot+string(os.PathSeparator))) {
				return nil, fmt.Errorf("%w：资源不存在或越界 %s", ErrSourceNotFound, rel)
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return nil, fmt.Errorf("%w：资源不存在 %s", ErrSourceNotFound, rel)
			}
			kind, primary, isDir, err := resolveKind(asset.Kind, resolved, info)
			if err != nil {
				return nil, fmt.Errorf("%w：%s", err, rel)
			}
			title := strings.TrimSpace(asset.Title)
			if title == "" {
				title = filepath.Base(primary)
			}
			fileName := filepath.Base(primary)
			if isDir {
				fileName = "index.html"
			}
			storageKey := storageKeyFor(m.CourseCode, lecture.No, kind, fileName)
			if seenKey[storageKey] {
				return nil, fmt.Errorf("%w：资源重复 %s", ErrInput, storageKey)
			}
			seenKey[storageKey] = true
			item := plannedAsset{
				lectureNo: lecture.No, kind: kind, title: title, sort: asset.Sort,
				sourcePath: resolved, storageKey: storageKey, isDir: isDir,
			}
			if kind == "html" && isDir {
				item.sourcePath = primary // 目录复制时保留目录作为源
			}
			if isDir {
				indexPath := filepath.Join(resolved, "index.html")
				if _, err := os.Stat(indexPath); err != nil {
					indexPath = filepath.Join(resolved, "index.htm")
					if _, err := os.Stat(indexPath); err != nil {
						return nil, fmt.Errorf("%w：HTML 目录缺少 index.html", ErrInput)
					}
				}
				info, _ = os.Stat(indexPath)
				item.size = info.Size()
				digest, _ := hashFile(indexPath)
				item.sha256 = digest
			}
			plan = append(plan, item)
		}
	}
	return plan, nil
}

func resolveKind(explicit, path string, info os.FileInfo) (kind, primary string, isDir bool, err error) {
	explicit = strings.TrimSpace(explicit)
	if info.IsDir() {
		if explicit != "" && explicit != "html" {
			return "", "", false, ErrUnsupported
		}
		return "html", path, true, nil
	}
	primary = path
	if explicit != "" {
		if !validKinds[explicit] {
			return "", "", false, ErrUnsupported
		}
		return explicit, primary, false, nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return "html", primary, false, nil
	case ".md", ".markdown":
		return "markdown", primary, false, nil
	case ".mp4", ".mov", ".m4v":
		return "video", primary, false, nil
	case ".mp3", ".wav", ".m4a", ".flac", ".aac":
		return "audio", primary, false, nil
	case ".pdf", ".ppt", ".pptx", ".key":
		return "slide", primary, false, nil
	case ".py", ".js", ".ts", ".go", ".java", ".c", ".cpp", ".rs", ".sh", ".sql":
		return "code", primary, false, nil
	case ".json":
		return "frozen_output", primary, false, nil
	default:
		return "", "", false, ErrUnsupported
	}
}

func storageKeyFor(code string, lectureNo int, kind, filename string) string {
	return strings.Join([]string{
		"courses", code, "lectures", strconv.Itoa(lectureNo), kind, filepath.ToSlash(filename),
	}, "/")
}

func (im *Importer) uploadAsset(ctx context.Context, item *plannedAsset) error {
	digest, err := hashFile(item.sourcePath)
	if err != nil {
		return err
	}
	file, err := os.Open(item.sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := im.Uploader.Upload(ctx, item.storageKey, file); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	item.size, item.sha256 = info.Size(), digest
	return nil
}

func copyWithHash(src, dst string) (int64, string, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, "", err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return 0, "", err
	}
	defer out.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(out, hash), in)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		_, _, err = copyWithHash(path, target)
		return err
	})
}

func cleanupLocal(paths []string) {
	for i := len(paths) - 1; i >= 0; i-- {
		_ = os.RemoveAll(paths[i])
	}
}

// DetectKind 按显式类型或扩展名判断单个文件的资源类型（供教师上传复用）。
func DetectKind(filename, explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		if !validKinds[explicit] {
			return "", ErrUnsupported
		}
		return explicit, nil
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html", ".htm":
		return "html", nil
	case ".md", ".markdown":
		return "markdown", nil
	case ".mp4", ".mov", ".m4v":
		return "video", nil
	case ".mp3", ".wav", ".m4a", ".flac", ".aac":
		return "audio", nil
	case ".pdf", ".ppt", ".pptx", ".key":
		return "slide", nil
	case ".py", ".js", ".ts", ".go", ".java", ".c", ".cpp", ".rs", ".sh", ".sql":
		return "code", nil
	case ".json":
		return "frozen_output", nil
	default:
		return "", ErrUnsupported
	}
}

// StorageKey 返回课程资源的标准对象键（与导入 /同步共用同一套目录规范）。
func StorageKey(courseCode string, lectureNo int, kind, filename string) string {
	return storageKeyFor(courseCode, lectureNo, kind, filename)
}
