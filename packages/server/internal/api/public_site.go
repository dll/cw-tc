package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/identity"
	"github.com/cw-tc/server/internal/redact"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

var contentBlocks = map[string]bool{
	"intro_short": true, "intro_para": true, "intro_long": true, "highlight": true,
	"ai_tech": true, "major_note": true, "claim_note": true, "filing_note": true,
}

func (s *Server) adminListContent(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, block, slot, COALESCE(title,''), body, COALESCE(evidence_ref,''), version, status,
		        COALESCE(published_at,''), updated_at FROM web_content
		 WHERE ($1='' OR status=$1) ORDER BY block, slot, version DESC`, strings.TrimSpace(c.Query("status")))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "对外内容读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var block, title, body, evidence, statusValue, publishedAt, updatedAt string
		var slot, version int
		if rows.Scan(&id, &block, &slot, &title, &body, &evidence, &version, &statusValue, &publishedAt, &updatedAt) == nil {
			items = append(items, gin.H{"id": id, "block": block, "slot": slot, "title": title, "body": body,
				"evidence_ref": evidence, "version": version, "status": statusValue,
				"published_at": publishedAt, "updated_at": updatedAt})
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminUpdateContent(c *gin.Context) {
	var in struct {
		Block       string `json:"block"`
		Slot        int    `json:"slot"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		EvidenceRef string `json:"evidence_ref"`
		Status      string `json:"status"`
		Confirm     bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !contentBlocks[in.Block] || strings.TrimSpace(in.Body) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "内容块或正文不合法"})
		return
	}
	if in.Status == "" {
		in.Status = "draft"
	}
	if in.Status != "draft" && in.Status != "published" && in.Status != "retired" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态只接受 draft / published / retired"})
		return
	}
	if in.Status == "published" && strings.TrimSpace(in.EvidenceRef) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "发布对外内容必须填写可核对依据"})
		return
	}
	if in.Block == "filing_note" && in.Status == "published" && strings.TrimSpace(in.Body) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "AI 公示不得为空"})
		return
	}
	if in.Status != "draft" && !in.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "发布 / 下线必须二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	tx, err := s.dbConn.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "保存失败"})
		return
	}
	defer tx.Rollback()
	var version int
	_ = tx.QueryRowContext(c.Request.Context(),
		`SELECT COALESCE(MAX(version),0)+1 FROM web_content WHERE block=$1 AND slot=$2`, in.Block, in.Slot).Scan(&version)
	if in.Status == "published" {
		if _, err := tx.ExecContext(c.Request.Context(),
			`UPDATE web_content SET status='retired' WHERE block=$1 AND slot=$2 AND status='published'`, in.Block, in.Slot); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "保存失败"})
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var publishedBy any
	var publishedAt any
	if in.Status == "published" {
		publishedBy, publishedAt = admin.ID, now
	}
	if _, err := tx.ExecContext(c.Request.Context(),
		`INSERT INTO web_content(block, slot, title, body, evidence_ref, version, status, published_by, published_at, updated_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		in.Block, in.Slot, nullIfBlank(in.Title), strings.TrimSpace(in.Body), nullIfBlank(in.EvidenceRef),
		version, in.Status, publishedBy, publishedAt, now); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "保存失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "保存失败"})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "content.update",
		TargetType: "web_content", TargetID: in.Block, After: in.Status, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"block": in.Block, "slot": in.Slot, "version": version, "status": in.Status}})
}

func (s *Server) adminListJobMap(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT j.id, j.course_code, c.name, j.cert_level, j.job_direction, j.skill_items, j.lecture_refs,
		        COALESCE(j.evidence_ref,''), j.status, COALESCE(j.major_id,0), COALESCE(j.reviewed_at,'')
		 FROM job_map j LEFT JOIN course c ON c.course_code=j.course_code
		 WHERE ($1='' OR j.status=$1) ORDER BY j.course_code, j.cert_level`, strings.TrimSpace(c.Query("status")))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "就业映射读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, majorID int64
		var code, name, level, direction, skills, refs, evidence, statusValue, reviewedAt string
		if rows.Scan(&id, &code, &name, &level, &direction, &skills, &refs, &evidence, &statusValue, &majorID, &reviewedAt) == nil {
			items = append(items, gin.H{"id": id, "course_code": code, "course_name": name, "cert_level": level,
				"job_direction": direction, "skill_items": jsonValue(skills), "lecture_refs": jsonValue(refs),
				"evidence_ref": evidence, "status": statusValue, "major_id": majorID, "reviewed_at": reviewedAt})
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminUpdateJobMap(c *gin.Context) {
	var in struct {
		ID           int64    `json:"id"`
		CourseCode   string   `json:"course_code"`
		CertLevel    string   `json:"cert_level"`
		JobDirection string   `json:"job_direction"`
		SkillItems   []string `json:"skill_items"`
		LectureRefs  []any    `json:"lecture_refs"`
		EvidenceRef  string   `json:"evidence_ref"`
		Status       string   `json:"status"`
		MajorID      *int64   `json:"major_id"`
		Confirm      bool     `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.CourseCode) == "" ||
		(in.CertLevel != "L1" && in.CertLevel != "L2" && in.CertLevel != "L3") ||
		strings.TrimSpace(in.JobDirection) == "" || len(in.SkillItems) == 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "课程 / 等级 / 岗位方向 / 能力项必填"})
		return
	}
	if in.Status == "" {
		in.Status = "draft"
	}
	if in.Status != "draft" && in.Status != "published" && in.Status != "retired" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "状态非法"})
		return
	}
	if in.Status == "published" && (strings.TrimSpace(in.EvidenceRef) == "" || len(in.LectureRefs) == 0) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "发布就业映射必须同时具备依据与讲次引用"})
		return
	}
	if in.Status != "draft" && !in.Confirm {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "发布 / 下线必须二次确认"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	skills, _ := json.Marshal(in.SkillItems)
	refs, _ := json.Marshal(in.LectureRefs)
	now := time.Now().UTC().Format(time.RFC3339)
	var id = in.ID
	if in.Status == "published" {
		_, _ = s.dbConn.ExecContext(c.Request.Context(),
			`UPDATE job_map SET status='retired' WHERE course_code=$1 AND cert_level=$2 AND status='published'`,
			in.CourseCode, in.CertLevel)
	}
	if id > 0 {
		_, err := s.dbConn.ExecContext(c.Request.Context(),
			`UPDATE job_map SET course_code=$1, cert_level=$2, job_direction=$3, skill_items=$4, lecture_refs=$5,
			   evidence_ref=$6, status=$7, major_id=$8, reviewed_by=$9, reviewed_at=$10 WHERE id=$11`,
			in.CourseCode, in.CertLevel, strings.TrimSpace(in.JobDirection), string(skills), string(refs),
			nullIfBlank(in.EvidenceRef), in.Status, in.MajorID, admin.ID, now, id)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "就业映射保存失败"})
			return
		}
	} else {
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`INSERT INTO job_map(course_code, cert_level, job_direction, skill_items, lecture_refs, evidence_ref,
			   status, major_id, reviewed_by, reviewed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
			in.CourseCode, in.CertLevel, strings.TrimSpace(in.JobDirection), string(skills), string(refs),
			nullIfBlank(in.EvidenceRef), in.Status, in.MajorID, admin.ID, now).Scan(&id); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "就业映射保存失败"})
			return
		}
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "job_map.update",
		TargetType: "job_map", TargetID: strconv.FormatInt(id, 10), After: in.Status, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id, "status": in.Status}})
}

func (s *Server) adminListShowcase(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, student_id, kind, nickname, COALESCE(cert_no,''), COALESCE(cert_level,''), summary,
		        COALESCE(job_direction,''), COALESCE(company_name,''), company_consent, status,
		        COALESCE(display_consent_at,''), COALESCE(consent_revoked_at,''), COALESCE(verified_at,''), created_at
		 FROM showcase_item WHERE ($1='' OR status=$1) ORDER BY created_at DESC LIMIT 100`, strings.TrimSpace(c.Query("status")))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "学员成果读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, studentID int64
		var kind, nickname, certNo, certLevel, summary, job, company, statusValue, consentAt, revokedAt, verifiedAt, createdAt string
		var companyConsent bool
		if rows.Scan(&id, &studentID, &kind, &nickname, &certNo, &certLevel, &summary, &job, &company,
			&companyConsent, &statusValue, &consentAt, &revokedAt, &verifiedAt, &createdAt) == nil {
			items = append(items, gin.H{"id": id, "student_id": studentID, "kind": kind, "nickname": nickname,
				"cert_no": certNo, "cert_level": certLevel, "summary": summary, "job_direction": job,
				"company_name": company, "company_consent": companyConsent, "status": statusValue,
				"display_consent_at": consentAt, "consent_revoked_at": revokedAt, "verified_at": verifiedAt, "created_at": createdAt})
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminUpdateShowcase(c *gin.Context) {
	var in struct {
		ID      int64  `json:"id"`
		Action  string `json:"action"`
		Note    string `json:"note"`
		Confirm bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.ID <= 0 || !in.Confirm ||
		(in.Action != "verify" && in.Action != "withdraw") {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "成果处理参数不合法"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	now := time.Now().UTC().Format(time.RFC3339)
	statusValue := "verified"
	if in.Action == "withdraw" {
		statusValue = "withdrawn"
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE showcase_item SET status=$1, verified_by=$2, verified_at=$3, updated_at=$3 WHERE id=$4`,
		statusValue, admin.ID, now, in.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "成果处理失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "成果不存在"})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role), Action: "showcase." + in.Action,
		TargetType: "showcase_item", TargetID: strconv.FormatInt(in.ID, 10), After: in.Note, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}

func (s *Server) myShowcase(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, kind, nickname, summary, COALESCE(job_direction,''), COALESCE(company_name,''), company_consent,
		        status, COALESCE(display_consent_at,''), COALESCE(consent_revoked_at,''), created_at
		 FROM showcase_item WHERE student_id=$1 ORDER BY created_at DESC`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "成果读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var kind, nickname, summary, job, company, statusValue, consentAt, revokedAt, createdAt string
		var companyConsent bool
		if rows.Scan(&id, &kind, &nickname, &summary, &job, &company, &companyConsent, &statusValue,
			&consentAt, &revokedAt, &createdAt) == nil {
			items = append(items, gin.H{"id": id, "kind": kind, "nickname": nickname, "summary": summary,
				"job_direction": job, "company_name": company, "company_consent": companyConsent,
				"status": statusValue, "display_consent_at": consentAt, "consent_revoked_at": revokedAt, "created_at": createdAt})
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) createShowcase(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var in struct {
		Kind           string `json:"kind"`
		Nickname       string `json:"nickname"`
		Summary        string `json:"summary"`
		JobDirection   string `json:"job_direction"`
		CompanyName    string `json:"company_name"`
		CompanyConsent bool   `json:"company_consent"`
		ContestName    string `json:"contest_name"`
		ContestYear    *int   `json:"contest_year"`
		AwardLevel     string `json:"award_level"`
		ProofNo        string `json:"proof_no"`
		TeamRole       string `json:"team_role"`
		EvidenceKind   string `json:"evidence_kind"`
		CertNo         string `json:"cert_no"`
		DisplayConsent bool   `json:"display_consent"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !validShowcaseKind(in.Kind) ||
		strings.TrimSpace(in.Nickname) == "" || strings.TrimSpace(in.Summary) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "成果类型 / 昵称 / 摘要必填"})
		return
	}
	if in.Nickname == student.Name || redact.ContainsIDCard(in.Summary) || redact.ContainsPhone(in.Summary) ||
		redact.ContainsIDCard(in.Nickname) || redact.ContainsPhone(in.Nickname) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "成果不得包含真实姓名 / 手机号 / 身份证号"})
		return
	}
	if in.CompanyName != "" && !in.CompanyConsent {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "展示企业名必须单独授权"})
		return
	}
	if in.Kind == "contest_award" && (strings.TrimSpace(in.ContestName) == "" || strings.TrimSpace(in.ProofNo) == "") {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "获奖成果必须填写赛事与凭证编号"})
		return
	}
	if in.Kind == "job_landed" && strings.TrimSpace(in.JobDirection) == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "就业成果必须填写岗位方向"})
		return
	}
	var certNo, certLevel any
	if in.Kind == "cert_earned" {
		var level string
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT cert_no, level FROM certificate WHERE student_id=$1 AND cert_no=$2 AND status='valid'`,
			student.ID, strings.TrimSpace(in.CertNo)).Scan(&in.CertNo, &level); err != nil {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "证书成果必须引用本人有效证书"})
			return
		}
		certNo, certLevel = in.CertNo, level
	}
	var consentAt any
	if in.DisplayConsent {
		consentAt = time.Now().UTC().Format(time.RFC3339)
	}
	var id int64
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`INSERT INTO showcase_item(student_id, kind, nickname, cert_no, cert_level, summary, job_direction,
		   company_name, company_consent, contest_name, contest_year, award_level, proof_no, team_role,
		   evidence_kind, display_consent_at, status)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'self_report') RETURNING id`,
		student.ID, in.Kind, strings.TrimSpace(in.Nickname), certNo, certLevel, strings.TrimSpace(in.Summary),
		nullIfBlank(in.JobDirection), nullIfBlank(in.CompanyName), in.CompanyConsent,
		nullIfBlank(in.ContestName), in.ContestYear, nullIfBlank(in.AwardLevel), nullIfBlank(in.ProofNo),
		nullIfBlank(in.TeamRole), nullIfBlank(in.EvidenceKind), consentAt).Scan(&id); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "成果提交失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id, "status": "self_report"}})
}

func (s *Server) revokeShowcase(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := s.dbConn.ExecContext(c.Request.Context(),
		`UPDATE showcase_item SET status='withdrawn', consent_revoked_at=$1, updated_at=$1
		 WHERE id=$2 AND student_id=$3`, time.Now().UTC().Format(time.RFC3339), id, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "撤回失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "成果不存在"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"status": "withdrawn"}})
}

func validShowcaseKind(kind string) bool {
	return kind == "job_landed" || kind == "contest_award" || kind == "cert_earned" || kind == "review_quote"
}

// publicOverview GET /public/overview —— 只读已发布内容；AI 公示缺失时 fail-closed。
func (s *Server) publicOverview(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT block, slot, COALESCE(title,''), body, COALESCE(evidence_ref,'' )
		 FROM web_content WHERE status='published' ORDER BY block, slot`)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "对外内容不可用"})
		return
	}
	blocks := make([]gin.H, 0)
	filingNote := ""
	for rows.Next() {
		var block, title, body, evidence string
		var slot int
		if rows.Scan(&block, &slot, &title, &body, &evidence) == nil {
			if block == "filing_note" {
				filingNote = body
			}
			blocks = append(blocks, gin.H{"block": block, "slot": slot, "title": title, "body": body, "evidence_ref": evidence})
		}
	}
	rows.Close()
	if strings.TrimSpace(filingNote) == "" {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10601, Msg: "AI 公示未配置，对外展示不可用"})
		return
	}
	items, _ := s.publicMajors(c)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"blocks": blocks, "majors": items, "filing_note": filingNote,
	}})
}

func (s *Server) publicMajors(c *gin.Context) ([]gin.H, error) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, name FROM major WHERE status='enabled' ORDER BY sort, id`)
	if err != nil {
		return []gin.H{}, err
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			items = append(items, gin.H{"id": id, "name": name})
		}
	}
	return items, rows.Err()
}

// publicJobMap GET /public/job-map —— 只回 published 且依据与讲次齐备的映射。
func (s *Server) publicJobMap(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT j.course_code, c.name, j.cert_level, j.job_direction, j.skill_items, j.lecture_refs, j.evidence_ref
		 FROM job_map j JOIN course c ON c.course_code=j.course_code
		 WHERE j.status='published' AND j.evidence_ref IS NOT NULL AND j.evidence_ref <> ''
		   AND j.lecture_refs <> '[]' ORDER BY j.course_code, j.cert_level`)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "就业映射不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, name, level, direction, skills, refs, evidence string
		if rows.Scan(&code, &name, &level, &direction, &skills, &refs, &evidence) == nil {
			items = append(items, gin.H{"course_code": code, "course_name": name, "cert_level": level,
				"job_direction": direction, "skill_items": jsonValue(skills),
				"lecture_refs": jsonValue(refs), "evidence_ref": evidence})
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// publicShowcase GET /public/showcase —— 只回 verified + 已授权未撤回；不含 student_id / 真实姓名。
func (s *Server) publicShowcase(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数不合法"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT kind, nickname, summary, COALESCE(job_direction,''), 
		        CASE WHEN company_consent THEN COALESCE(company_name,'') ELSE '' END,
		        COALESCE(cert_level,''), COALESCE(contest_name,''), COALESCE(contest_year,0),
		        COALESCE(award_level,''), COALESCE(team_role,''), COALESCE(display_consent_at,'')
		 FROM showcase_item
		 WHERE status='verified' AND display_consent_at IS NOT NULL AND consent_revoked_at IS NULL
		 ORDER BY verified_at DESC, id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "学员成果不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var kind, nickname, summary, job, company, certLevel, contest, award, team, consentAt string
		var year int
		if rows.Scan(&kind, &nickname, &summary, &job, &company, &certLevel, &contest, &year, &award, &team, &consentAt) == nil {
			items = append(items, gin.H{"kind": kind, "nickname": nickname, "summary": summary,
				"job_direction": job, "company_name": company, "cert_level": certLevel,
				"contest_name": contest, "contest_year": year, "award_level": award, "team_role": team})
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
