package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

func (s *Server) adminBatchCourses(c *gin.Context) {
	var in struct {
		Action      string   `json:"action"`
		CourseCodes []string `json:"course_codes"`
		PriceCents  *int64   `json:"price_cents"`
		MajorID     *int64   `json:"major_id"`
		Reason      string   `json:"reason"`
		Preview     bool     `json:"preview"`
		Confirm     bool     `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || (in.Action != "price" && in.Action != "major") ||
		len(in.CourseCodes) == 0 || len(in.CourseCodes) > 100 || (!in.Preview && !in.Confirm) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "批量操作仅支持 price / major，单次 ≤100，执行需二次确认"})
		return
	}
	if in.Action == "price" && (in.PriceCents == nil || *in.PriceCents < 0) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "价格不能为负"})
		return
	}
	if in.Action == "major" && (in.MajorID == nil || *in.MajorID <= 0) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "专业必填"})
		return
	}
	if in.Action == "major" {
		var exists int
		if err := s.dbConn.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM major WHERE id=$1`, *in.MajorID).Scan(&exists); err != nil || exists == 0 {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "专业不存在"})
			return
		}
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	seen := map[string]bool{}
	results := make([]gin.H, 0, len(in.CourseCodes))
	now := time.Now().UTC().Format(time.RFC3339)
	for _, raw := range in.CourseCodes {
		code := strings.TrimSpace(raw)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		var count int
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT COUNT(*) FROM course WHERE course_code=$1 AND deleted_at IS NULL`, code).Scan(&count); err != nil || count == 0 {
			results = append(results, gin.H{"course_code": code, "ok": false, "reason": "课程不存在"})
			continue
		}
		if in.Preview {
			results = append(results, gin.H{"course_code": code, "ok": true, "would_apply": true})
			continue
		}
		var err error
		if in.Action == "price" {
			_, err = s.dbConn.ExecContext(c.Request.Context(),
				`UPDATE course SET price_cents=$1, updated_at=$2 WHERE course_code=$3 AND deleted_at IS NULL`,
				*in.PriceCents, now, code)
		} else {
			_, err = s.dbConn.ExecContext(c.Request.Context(),
				`UPDATE course SET major_id=$1, updated_at=$2 WHERE course_code=$3 AND deleted_at IS NULL`,
				*in.MajorID, now, code)
		}
		if err != nil {
			results = append(results, gin.H{"course_code": code, "ok": false, "reason": "写入失败"})
			continue
		}
		results = append(results, gin.H{"course_code": code, "ok": true})
		after := `{"action":"` + in.Action + `","reason":"` + strings.ReplaceAll(strings.TrimSpace(in.Reason), `"`, `'`) + `"}`
		_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
			ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "batch." + in.Action,
			TargetType: "course", TargetID: code, After: after, IP: c.ClientIP(),
		})
	}
	batchID := newBatchID()
	rawResults, _ := json.Marshal(results)
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO batch_job(id, action, operator_id, preview, results, created_at) VALUES($1,$2,$3,$4,$5,$6)`,
		batchID, in.Action, admin.ID, in.Preview, string(rawResults), now); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "批量结果保存失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"batch_id": batchID, "preview": in.Preview, "action": in.Action, "items": results,
	}})
}

func (s *Server) adminBatchResult(c *gin.Context) {
	var action, raw string
	var preview bool
	var createdAt string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT action, preview, results, created_at FROM batch_job WHERE id=$1`, c.Param("batchId")).
		Scan(&action, &preview, &raw, &createdAt); err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "批量任务不存在"})
		return
	}
	var items any
	_ = json.Unmarshal([]byte(raw), &items)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"batch_id": c.Param("batchId"), "action": action, "preview": preview,
		"items": items, "created_at": createdAt,
	}})
}

func newBatchID() string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return fmt.Sprintf("B%s%s", strconv.FormatInt(time.Now().UnixNano(), 36), hex.EncodeToString(raw[:]))
}
