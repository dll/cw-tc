package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/cw-tc/server/internal/audit"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

// adminListAdmins GET /admin/admins —— 管理员列表（仅 super）。
func (s *Server) adminListAdmins(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT id, account, name, role, status, created_at FROM staff
		 WHERE role IN ('admin_super','admin_ops') ORDER BY role, id`)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "管理员列表读取失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var account, name, role, status, createdAt string
		if err := rows.Scan(&id, &account, &name, &role, &status, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "管理员列表读取失败"})
			return
		}
		items = append(items, gin.H{
			"id": id, "account": account, "name": name, "role": role,
			"status": status, "created_at": createdAt,
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// adminCreateAdmin POST /admin/admins —— 创建管理员 / 超管（仅 super，写审计）。
func (s *Server) adminCreateAdmin(c *gin.Context) {
	var req struct {
		Account  string `json:"account"`
		Name     string `json:"name"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil ||
		(req.Role != string(staff.RoleAdminSuper) && req.Role != string(staff.RoleAdminOps)) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "角色只接受 admin_ops / admin_super"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	id, err := staff.Create(c.Request.Context(), s.dbConn, req.Account, req.Name, staff.Role(req.Role), req.Password, admin.ID)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "admin.create", TargetType: "staff", TargetID: strconv.FormatInt(id, 10),
		After: `{"role":"` + req.Role + `"}`, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id}})
}

// adminDisableAdmin DELETE /admin/admins/:id —— 停用管理员（仅 super；禁止停用自己与最后一个超管）。
func (s *Server) adminDisableAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "管理员编号非法"})
		return
	}
	admin := c.MustGet(staffContextKey).(staff.Staff)
	if id == admin.ID {
		c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "不能停用自己"})
		return
	}
	var role, status string
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT role, status FROM staff WHERE id = $1`, id).Scan(&role, &status); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "管理员不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "管理员读取失败"})
		return
	}
	if role != string(staff.RoleAdminSuper) && role != string(staff.RoleAdminOps) {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "该账号不是管理员"})
		return
	}
	if role == string(staff.RoleAdminSuper) {
		var supers int
		if err := s.dbConn.QueryRowContext(c.Request.Context(),
			`SELECT COUNT(*) FROM staff WHERE role = 'admin_super' AND status = 'active'`).Scan(&supers); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "管理员读取失败"})
			return
		}
		if supers <= 1 {
			c.JSON(http.StatusConflict, envelope{Code: 10409, Msg: "不能停用最后一个超管"})
			return
		}
	}
	if err := staff.Disable(c.Request.Context(), s.dbConn, id); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10802, Msg: err.Error()})
		return
	}
	_ = audit.Record(c.Request.Context(), s.dbConn, audit.Input{
		ActorType: "staff", ActorID: admin.ID, ActorRole: string(admin.Role),
		Action: "admin.disable", TargetType: "staff", TargetID: strconv.FormatInt(id, 10),
		After: `{"status":"disabled","role":"` + role + `"}`, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, envelope{Code: 0})
}
