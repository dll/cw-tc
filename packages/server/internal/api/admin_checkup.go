package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// adminCourseCheckup GET /admin/courses/:code/checkup —— 平台课程内容体检（不替代审核）。
func (s *Server) adminCourseCheckup(c *gin.Context) {
	code := c.Param("code")
	var courseID int64
	var name string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT id, name FROM course WHERE course_code=$1 AND deleted_at IS NULL`, code).Scan(&courseID, &name); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	var lectureCount, lecturesWithoutAsset int
	_ = s.dbConn.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM lecture WHERE course_id=$1`, courseID).Scan(&lectureCount)
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM lecture l WHERE l.course_id=$1 AND NOT EXISTS(SELECT 1 FROM asset a WHERE a.lecture_id=l.id)`, courseID).Scan(&lecturesWithoutAsset)
	assetCounts := map[string]int{}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT a.kind, COUNT(*) FROM asset a JOIN lecture l ON l.id=a.lecture_id WHERE l.course_id=$1 GROUP BY a.kind`, courseID)
	if err == nil {
		for rows.Next() {
			var kind string
			var count int
			if rows.Scan(&kind, &count) == nil {
				assetCounts[kind] = count
			}
		}
		rows.Close()
	}
	var questionCount, duplicateQno int
	_ = s.dbConn.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM question WHERE course_id=$1`, courseID).Scan(&questionCount)
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM (SELECT qno FROM question WHERE course_id=$1 GROUP BY qno HAVING COUNT(*)>1) t`, courseID).Scan(&duplicateQno)
	levelCounts := map[string]int{}
	levelRows, _ := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT level, COUNT(*) FROM question WHERE course_id=$1 GROUP BY level`, courseID)
	if levelRows != nil {
		for levelRows.Next() {
			var level string
			var count int
			if levelRows.Scan(&level, &count) == nil {
				levelCounts[level] = count
			}
		}
		levelRows.Close()
	}
	missingFiles := 0
	if s.cfg != nil && s.cfg.ContentDir != "" {
		fileRows, _ := s.dbConn.QueryContext(c.Request.Context(),
			`SELECT a.storage_key FROM asset a JOIN lecture l ON l.id=a.lecture_id
			 WHERE l.course_id=$1 AND a.kind IN ('markdown','html','slide','code')`, courseID)
		if fileRows != nil {
			for fileRows.Next() {
				var key string
				if fileRows.Scan(&key) == nil {
					if _, err := os.Stat(filepath.Join(s.cfg.ContentDir, filepath.FromSlash(key))); err != nil {
						missingFiles++
					}
				}
			}
			fileRows.Close()
		}
	}
	check := func(label string, passed bool, detail string) gin.H {
		return gin.H{"label": label, "passed": passed, "detail": detail}
	}
	hasLecture := lectureCount > 0
	allHaveAssets := lectureCount > 0 && lecturesWithoutAsset == 0
	hasText := assetCounts["markdown"]+assetCounts["html"] > 0
	hasMedia := assetCounts["video"]+assetCounts["audio"]+assetCounts["slide"] > 0
	hasQuestions := questionCount > 0
	noDup := duplicateQno == 0
	items := []gin.H{
		check("讲次完整", hasLecture, "讲次数量 "+itoaInt(lectureCount)),
		check("每讲有资源", allHaveAssets, "缺资源讲次 "+itoaInt(lecturesWithoutAsset)),
		check("讲义 / 课件", hasText, "markdown="+itoaInt(assetCounts["markdown"])+" html="+itoaInt(assetCounts["html"])),
		check("媒体 / 课件", hasMedia, "video="+itoaInt(assetCounts["video"])+" audio="+itoaInt(assetCounts["audio"])+" slide="+itoaInt(assetCounts["slide"])),
		check("题库非空", hasQuestions, "题目 "+itoaInt(questionCount)),
		check("题库无重复编号", noDup, "重复题号 "+itoaInt(duplicateQno)),
		check("资源文件齐备", missingFiles == 0, "缺失文件 "+itoaInt(missingFiles)),
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"course_code": code, "name": name, "items": items,
		"asset_counts": assetCounts, "question_levels": levelCounts,
		"ready": hasLecture && allHaveAssets && hasText && hasQuestions && noDup && missingFiles == 0,
		"note":  "体检只挡明显缺项，不替代教师 / 管理员审核",
	}})
}

func itoaInt(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	buf := make([]byte, 0, 12)
	for value > 0 {
		buf = append([]byte{byte('0' + value%10)}, buf...)
		value /= 10
	}
	if negative {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
