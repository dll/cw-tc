package api

import (
	"html/template"
	"net/http"

	"github.com/cw-tc/server/internal/exam"
	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// certHTML 可打印证书页模板（学员本人证书下载/打印）。
// 标准表述只有一种：畅网智能科技有限公司颁发的企业能力认证（铁律 13）。
var certHTML = template.Must(template.New("cert").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<title>能力认证证书</title>
<style>
  body{font-family:"PingFang SC","Microsoft YaHei",sans-serif;background:#f4f6f9;margin:0;padding:32px;color:#222}
  .card{max-width:760px;margin:0 auto;background:#fff;border:1px solid #e2e6ee;border-radius:12px;padding:48px;box-shadow:0 6px 24px rgba(0,0,0,.06)}
  .head{text-align:center;border-bottom:2px solid #2b5bb7;padding-bottom:20px;margin-bottom:28px}
  .head h1{font-size:26px;margin:0 0 8px;color:#1f3a6b}
  .head p{margin:0;color:#5a6b8c;font-size:13px}
  .field{margin:14px 0;font-size:16px}
  .field .k{color:#5a6b8c;display:inline-block;width:96px}
  .field .v{font-weight:600}
  .certno{font-family:ui-monospace,Consolas,monospace;letter-spacing:.5px}
  .foot{margin-top:32px;padding-top:16px;border-top:1px solid #e2e6ee;text-align:center;color:#5a6b8c;font-size:12px}
  @media print{body{background:#fff;padding:0}.card{border:none;box-shadow:none}}
</style></head><body>
<div class="card">
  <div class="head"><h1>能力认证证书</h1><p>畅网智能科技有限公司颁发的企业能力认证</p></div>
  <div class="field"><span class="k">学员姓名</span><span class="v">{{.StudentName}}</span></div>
  <div class="field"><span class="k">课程名称</span><span class="v">{{.CourseName}}{{if .LevelName}}（{{.LevelName}}）{{end}}</span></div>
  <div class="field"><span class="k">证书编号</span><span class="v certno">{{.CertNo}}</span></div>
  <div class="field"><span class="k">签发日期</span><span class="v">{{.IssuedAt}}</span></div>
  <div class="field"><span class="k">有效期至</span><span class="v">{{.ExpiresAt}}</span></div>
  <div class="foot">本证书可通过平台验真页查询真伪 · 证书编号唯一</div>
</div>
</body></html>`))

// downloadMyCert GET /me/certs/:certNo —— 学员本人证书的可打印 HTML（完整姓名，仅本人可见）。
func (s *Server) downloadMyCert(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	cert, err := exam.GetOwnCert(c.Request.Context(), s.dbConn, student.ID, c.Param("certNo"))
	if err != nil {
		if err == exam.ErrCertNotFound {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "证书不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	_ = certHTML.Execute(c.Writer, map[string]string{
		"StudentName": cert.StudentName,
		"CourseName":  cert.CourseName,
		"LevelName":   cert.LevelName,
		"CertNo":      cert.CertNo,
		"IssuedAt":    cert.IssuedAt.Format("2006-01-02"),
		"ExpiresAt":   cert.ExpiresAt.Format("2006-01-02"),
	})
}

// listMyCerts GET /me/certs —— 学员自己的证书列表（含课程码/名称）。
func (s *Server) listMyCerts(c *gin.Context) {
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
		return
	}
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT c.cert_no, co.course_code, co.name, c.level, c.score, c.band, c.status, c.issued_at, c.expires_at
		 FROM certificate c JOIN course co ON c.course_id = co.id
		 WHERE c.student_id = $1 ORDER BY c.issued_at DESC`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var certNo, courseCode, courseName, level, band, status, issuedAt, expiresAt string
		var score float64
		if err := rows.Scan(&certNo, &courseCode, &courseName, &level, &score, &band, &status, &issuedAt, &expiresAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
			return
		}
		items = append(items, gin.H{
			"cert_no": certNo, "course_code": courseCode, "course_name": courseName,
			"level": level, "score": score, "band": band, "status": status,
			"issued_at": issuedAt, "expires_at": expiresAt,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "证书服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
