// mediasync 把本地内容源里的视频/音频文件上传到 COS，并产出 asset 清单。
//
// 用法（在内容源所在机器上）：
//
//	CWTC_COS_SECRET_ID=… CWTC_COS_SECRET_KEY=… CWTC_COS_BUCKET=… CWTC_COS_REGION=… \
//	  mediasync --source <内容源目录> --manifest media.json
//
// 清单交给服务器侧 `sync --media-manifest media.json` 写入 asset 表（视频/音频走 COS）。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cw-tc/server/internal/cosstore"
	"github.com/cw-tc/server/internal/sync"
)

var lectureDirRe = regexp.MustCompile(`^(\d{1,2})_?(.+)$`)

var videoExt = map[string]bool{".mp4": true, ".mov": true, ".m4v": true}
var audioExt = map[string]bool{".mp3": true, ".wav": true, ".m4a": true, ".flac": true, ".aac": true}

type overviewCourse struct {
	ID  string `json:"id"`
	Dir string `json:"dir"`
}

// scanMedia 扫描内容源里的视频/音频文件，产出 asset 清单（纯函数，可单测）。
func scanMedia(source string) ([]sync.MediaEntry, error) {
	raw, err := os.ReadFile(filepath.Join(source, "课程总览.json"))
	if err != nil {
		return nil, err
	}
	var ov struct {
		Courses []overviewCourse `json:"courses"`
	}
	if err := json.Unmarshal(raw, &ov); err != nil {
		return nil, err
	}
	codeByDir := make(map[string]string, len(ov.Courses))
	for _, c := range ov.Courses {
		codeByDir[c.Dir] = c.ID
	}
	var entries []sync.MediaEntry
	for dir, code := range codeByDir {
		courseDir := filepath.Join(source, dir)
		lectureDirs, err := os.ReadDir(courseDir)
		if err != nil {
			continue
		}
		for _, ld := range lectureDirs {
			if !ld.IsDir() {
				continue
			}
			m := lectureDirRe.FindStringSubmatch(ld.Name())
			if m == nil {
				continue
			}
			no, err := strconv.Atoi(m[1])
			if err != nil || no < 1 {
				continue
			}
			lectureDir := filepath.Join(courseDir, ld.Name())
			_ = filepath.WalkDir(lectureDir, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					// 跳过 build/ 等生成中间产物目录（分段 video/tts 素材不是课程资源）
					if d.Name() == "build" {
						return filepath.SkipDir
					}
					return nil
				}
				ext := strings.ToLower(filepath.Ext(d.Name()))
				kind := ""
				switch {
				case videoExt[ext]:
					kind = "video"
				case audioExt[ext]:
					kind = "audio"
				default:
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return nil
				}
				rel, err := filepath.Rel(source, p)
				if err != nil {
					return nil
				}
				entries = append(entries, sync.MediaEntry{
					StorageKey: filepath.ToSlash(rel),
					Kind:       kind,
					Size:       info.Size(),
					CourseCode: code,
					LectureNo:  no,
				})
				return nil
			})
		}
	}
	return entries, nil
}

// uploader 抽象上传动作（cosstore.Store 满足），便于单测注入假上传器。
type uploader interface {
	Upload(ctx context.Context, key string, r io.Reader) error
}

// runUpload 逐个上传媒体文件，只返回确实已上传的资源，避免清单引用不存在的对象。
func runUpload(ctx context.Context, source string, entries []sync.MediaEntry, up uploader) []sync.MediaEntry {
	uploaded := make([]sync.MediaEntry, 0, len(entries))
	for _, e := range entries {
		f, err := os.Open(filepath.Join(source, filepath.FromSlash(e.StorageKey)))
		if err != nil {
			continue
		}
		if err := up.Upload(ctx, e.StorageKey, f); err != nil {
			_ = f.Close()
			continue
		}
		_ = f.Close()
		uploaded = append(uploaded, e)
	}
	return uploaded
}

// run 扫描媒体 → 上传 → 写清单（up 为 nil 时视为未配置，返回错误）。
func run(source, manifestPath string, up uploader) (int, error) {
	entries, err := scanMedia(source)
	if err != nil {
		return 0, err
	}
	if up == nil {
		return 0, errors.New("COS 未配置（CWTC_COS_SECRET_ID/KEY/BUCKET 缺一）")
	}
	uploaded := runUpload(context.Background(), source, entries, up)
	if manifestPath != "" {
		raw, _ := json.MarshalIndent(uploaded, "", "  ")
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			return 0, err
		}
	}
	if len(uploaded) != len(entries) {
		return len(uploaded), fmt.Errorf("媒体上传不完整：成功 %d/%d；仅成功资源写入清单", len(uploaded), len(entries))
	}
	return len(uploaded), nil
}

func main() {
	source := flag.String("source", "", "内容源目录（含课程总览.json）")
	manifestPath := flag.String("manifest", "", "asset 清单输出文件（JSON）")
	flag.Parse()
	if *source == "" {
		fmt.Fprintln(os.Stderr, "缺少 --source")
		os.Exit(2)
	}
	store := cosstore.New(os.Getenv("CWTC_COS_SECRET_ID"), os.Getenv("CWTC_COS_SECRET_KEY"), os.Getenv("CWTC_COS_BUCKET"), os.Getenv("CWTC_COS_REGION"))
	n, err := run(*source, *manifestPath, store)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("扫描媒体文件，上传成功 %d 个，清单写入 %s\n", n, *manifestPath)
}
