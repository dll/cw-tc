package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cw-tc/server/internal/ledger"
	"github.com/cw-tc/server/internal/staff"
	"github.com/gin-gonic/gin"
)

const staffContextKey = "staff"

// adminLogin POST /admin/login —— 内部人员登录，返回 Bearer token。
func (s *Server) adminLogin(c *gin.Context) {
	var req struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if s.dbConn == nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	st, token, err := staff.Login(c.Request.Context(), s.dbConn, req.Account, req.Password, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "账号或密码错误"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"token": token, "account": st.Account, "name": st.Name, "role": string(st.Role),
	}})
}

// staffRequired 内部人员鉴权：校验 Bearer token，注入 staff 到上下文。
func (s *Server) staffRequired(c *gin.Context) {
	token := bearerToken(c)
	if token == "" || s.dbConn == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
		return
	}
	st, err := staff.Authenticate(c.Request.Context(), s.dbConn, token, time.Now().UTC())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, envelope{Code: 10701, Msg: "未认证"})
		return
	}
	c.Set(staffContextKey, st)
	c.Next()
}

// requireSuper 超管专属：只有 admin_super 可过（铁律 8：高危操作后端二次确认 + 分级）。
func (s *Server) requireSuper(c *gin.Context) {
	v, ok := c.Get(staffContextKey)
	sst, ok2 := v.(staff.Staff)
	if !ok || !ok2 || sst.Role != staff.RoleAdminSuper {
		c.AbortWithStatusJSON(http.StatusForbidden, envelope{Code: 10703, Msg: "无权限"})
		return
	}
	c.Next()
}

func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}

func (s *Server) adminOrders(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, err := ledger.ListOrders(c.Request.Context(), s.dbConn, c.Query("status"), limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminRefunds(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, err := ledger.ListRefunds(c.Request.Context(), s.dbConn, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

func (s *Server) adminCerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, err := ledger.ListCertificates(c.Request.Context(), s.dbConn, limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
