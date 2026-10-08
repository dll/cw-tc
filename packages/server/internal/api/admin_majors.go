package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// adminListMajors GET /admin/majors —— 全部专业分类（含停用），平台自有数据（铁律 1 反向边界）。
func (s *Server) adminListMajors(c *gin.Context) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(), `SELECT id, name, sort, status FROM major ORDER BY sort ASC, id ASC`)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, sort int64
		var name, status string
		if err := rows.Scan(&id, &name, &sort, &status); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
			return
		}
		items = append(items, gin.H{"id": id, "name": name, "sort": sort, "status": status})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "服务不可用"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}

// adminCreateMajor POST /admin/majors —— 新建专业分类。
func (s *Server) adminCreateMajor(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		Sort int64  `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	res, err := s.dbConn.ExecContext(c.Request.Context(), `INSERT INTO major(name, sort, status) VALUES(?, ?, 'enabled')`, req.Name, req.Sort)
	if err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10002, Msg: "创建失败"})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"id": id}})
}

// adminUpdateMajor PUT /admin/majors/:id —— 修改专业分类（改名/排序/停用/启用）。
func (s *Server) adminUpdateMajor(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	var req struct {
		Name   string `json:"name"`
		Sort   int64  `json:"sort"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "参数错误"})
		return
	}
	if req.Status == "" {
		req.Status = "enabled"
	}
	if _, err := s.dbConn.ExecContext(c.Request.Context(), `UPDATE major SET name = COALESCE(NULLIF(?, ''), name), sort = ?, status = ? WHERE id = ?`, req.Name, req.Sort, req.Status, id); err != nil {
		c.JSON(http.StatusConflict, envelope{Code: 10002, Msg: "更新失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0})
}
