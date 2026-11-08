package courseimport

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ErrPackage = errors.New("课程资源包不合法")

const (
	// MaxPackageFiles 限制单个资源包内的文件数，避免恶意压缩包拖垮服务。
	MaxPackageFiles = 5000
	// MaxPackageBytes 限制解压后的总大小（防 zip bomb）。
	MaxPackageBytes = int64(2 << 30)
	// MaxPackageFileBytes 限制单个文件解压后的大小。
	MaxPackageFileBytes = int64(1 << 30)
)

// ExtractPackage 把 ZIP 课程资源包安全解压到 destDir，返回含 manifest.json 的根目录。
//
// 约定：manifest.json 位于包根目录；若整包外面又套了一层目录，也允许
// （仅当唯一顶层目录中存在 manifest.json 时）。
func ExtractPackage(zipPath, destDir string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("%w：不是有效的 ZIP 文件", ErrPackage)
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	total := int64(0)
	files := 0
	for _, entry := range reader.File {
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		name = strings.TrimPrefix(name, "./")
		if name == "" || strings.HasPrefix(name, "__MACOSX/") || path.Base(name) == ".DS_Store" {
			continue
		}
		clean := path.Clean(name)
		if clean == "." {
			continue
		}
		if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
			return "", fmt.Errorf("%w：非法路径 %s", ErrPackage, entry.Name)
		}
		target := filepath.Join(destDir, filepath.FromSlash(clean))
		if !withinDir(destDir, target) {
			return "", fmt.Errorf("%w：非法路径 %s", ErrPackage, entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			continue
		}
		files++
		if files > MaxPackageFiles {
			return "", fmt.Errorf("%w：文件数超过上限 %d", ErrPackage, MaxPackageFiles)
		}
		if entry.UncompressedSize64 > uint64(MaxPackageFileBytes) {
			return "", fmt.Errorf("%w：单个文件超过上限 %s", ErrPackage, entry.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		written, err := extractFile(entry, target)
		if err != nil {
			return "", err
		}
		total += written
		if total > MaxPackageBytes {
			return "", fmt.Errorf("%w：解压后总大小超过上限", ErrPackage)
		}
	}
	return findManifestRoot(destDir)
}

// ReadManifest 读取资源包根目录下的 manifest.json。
func ReadManifest(root string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("%w：根目录缺少 manifest.json", ErrPackage)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w：manifest.json 不是合法 JSON（%v）", ErrPackage, err)
	}
	return manifest, nil
}

func extractFile(entry *zip.File, target string) (int64, error) {
	source, err := entry.Open()
	if err != nil {
		return 0, err
	}
	defer source.Close()
	destination, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, MaxPackageFileBytes+1))
	closeErr := destination.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return 0, copyErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if written > MaxPackageFileBytes {
		_ = os.Remove(target)
		return 0, fmt.Errorf("%w：单个文件超过上限 %s", ErrPackage, entry.Name)
	}
	return written, nil
}

func findManifestRoot(destDir string) (string, error) {
	if isFile(filepath.Join(destDir, "manifest.json")) {
		return destDir, nil
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return "", err
	}
	dirs := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 1 {
		root := filepath.Join(destDir, dirs[0])
		if isFile(filepath.Join(root, "manifest.json")) {
			return root, nil
		}
	}
	return "", fmt.Errorf("%w：根目录缺少 manifest.json", ErrPackage)
}

func withinDir(base, target string) bool {
	cleanBase := filepath.Clean(base)
	cleanTarget := filepath.Clean(target)
	return cleanTarget == cleanBase || strings.HasPrefix(cleanTarget, cleanBase+string(os.PathSeparator))
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
