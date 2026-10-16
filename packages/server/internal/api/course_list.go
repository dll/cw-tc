package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 分页默认值与上限（契约 §1.3：page 从 1 起、page_size 默认 20、上限 100）。
const (
	courseDefaultPageSize = 20
	courseMaxPageSize     = 100
)

// listPublicCourses GET /courses 课程广场（公开只读）。
//
// 支持筛选 `major_id` / `line`(main|aux) / `q`（按名称或简介模糊匹配），
// 分页 `page` / `page_size`；**只返回已上架**（`status='published'`）。
func (s *Server) listPublicCourses(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}

	page, err := parsePageParam(c.Query("page"), 1)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "page 格式非法"})
		return
	}
	pageSize, err := parsePageParam(c.Query("page_size"), courseDefaultPageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "page_size 格式非法"})
		return
	}
	if page < 1 || pageSize < 1 || pageSize > courseMaxPageSize {
		c.JSON(http.StatusBadRequest, envelope{Code: 10003, Msg: "分页参数越界"})
		return
	}

	where := []string{"c.status = 'published'"}
	args := make([]any, 0, 4)

	if majorStr := strings.TrimSpace(c.Query("major_id")); majorStr != "" {
		majorID, err := strconv.ParseInt(majorStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "major_id 格式非法"})
			return
		}
		args = append(args, majorID)
		where = append(where, "c.major_id = $"+strconv.Itoa(len(args)))
	}

	if line := strings.TrimSpace(c.Query("line")); line != "" {
		if line != "main" && line != "aux" {
			c.JSON(http.StatusBadRequest, envelope{Code: 10002, Msg: "line 只接受 main 或 aux"})
			return
		}
		args = append(args, line)
		where = append(where, "c.line = $"+strconv.Itoa(len(args)))
	}

	if q := strings.TrimSpace(c.Query("q")); q != "" {
		// 转义 LIKE 通配符，避免用户输入被当作模式（契约 §1.3：不提供通用查询 DSL）。
		args = append(args, "%"+escapeLikePattern(q)+"%")
		idx := strconv.Itoa(len(args))
		where = append(where, "(c.name LIKE $"+idx+" ESCAPE '\\' OR c.intro LIKE $"+idx+" ESCAPE '\\')")
	}

	clause := " WHERE " + strings.Join(where, " AND ")
	ctx := c.Request.Context()

	var total int64
	if err := s.dbConn.QueryRowContext(ctx, `SELECT COUNT(*) FROM course c`+clause, args...).Scan(&total); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}

	listArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	limitIdx := strconv.Itoa(len(listArgs) - 1)
	offsetIdx := strconv.Itoa(len(listArgs))
	query := `SELECT c.course_code, c.name, c.intro, c.price_cents, c.original_price_cents, c.major_id, m.name, c.line, c.lecture_count, c.free_preview_lectures` +
		` FROM course c LEFT JOIN major m ON m.id = c.major_id` + clause +
		` ORDER BY c.sort_weight DESC, c.id ASC LIMIT $` + limitIdx + ` OFFSET $` + offsetIdx

	rows, err := s.dbConn.QueryContext(ctx, query, listArgs...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var code, name, line string
		var intro *string
		var price int64
		var original *int64
		var majorID *int64
		var majorName *string
		var lectureCount, freePreview int
		if err := rows.Scan(&code, &name, &intro, &price, &original, &majorID, &majorName, &line, &lectureCount, &freePreview); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
			return
		}
		items = append(items, gin.H{
			"course_code":           code,
			"name":                  name,
			"intro":                 intro,
			"price_cents":           price,
			"original_price_cents":  original,
			"major_id":              majorID,
			"major_name":            majorName,
			"line":                  line,
			"lecture_count":         lectureCount,
			"free_preview_lectures": freePreview,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "课程数据不可用"})
		return
	}

	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

// parsePageParam 解析分页整型参数；空串用默认值，非数字报错。
func parsePageParam(raw string, def int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}

// escapeLikePattern 转义 LIKE 模式中的通配符（\ % _），配合 `ESCAPE '\'` 使用。
func escapeLikePattern(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
