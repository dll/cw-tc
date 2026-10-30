package api

import (
	"database/sql"
	"net/http"

	"github.com/cw-tc/server/internal/identity"
	"github.com/gin-gonic/gin"
)

// favoriteStatus GET /courses/:code/favorite —— 当前学员是否已收藏。
func (s *Server) favoriteStatus(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var n int
	_ = s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM favorite WHERE student_id=$1 AND course_code=$2`,
		student.ID, c.Param("courseCode")).Scan(&n)
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"favorited": n > 0}})
}

// addFavorite POST /courses/:code/favorite —— 收藏（重复收藏幂等）。
func (s *Server) addFavorite(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	var exists int
	if err := s.dbConn.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM course WHERE course_code=$1 AND deleted_at IS NULL`, c.Param("courseCode")).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 10404, Msg: "课程不存在"})
		return
	}
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`INSERT INTO favorite(student_id, course_code) VALUES($1,$2) ON CONFLICT(student_id, course_code) DO NOTHING`,
		student.ID, c.Param("courseCode")); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "收藏失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"favorited": true}})
}

// removeFavorite DELETE /courses/:code/favorite —— 取消收藏（幂等）。
func (s *Server) removeFavorite(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	if _, err := s.dbConn.ExecContext(c.Request.Context(),
		`DELETE FROM favorite WHERE student_id=$1 AND course_code=$2`,
		student.ID, c.Param("courseCode")); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "取消收藏失败"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"favorited": false}})
}

// listFavorites GET /me/favorites —— 我的收藏（下架课程保留并标注）。
func (s *Server) listFavorites(c *gin.Context) {
	student := c.MustGet(studentContextKey).(identity.Student)
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT f.course_code, c.name, c.intro, c.price_cents, m.name, c.status,
		        EXISTS(SELECT 1 FROM entitlement e WHERE e.student_id=$1 AND e.course_id=c.id AND e.revoked_at IS NULL),
		        f.created_at
		 FROM favorite f JOIN course c ON c.course_code=f.course_code
		 LEFT JOIN major m ON m.id=c.major_id
		 WHERE f.student_id=$1 ORDER BY f.created_at DESC`, student.ID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "收藏查询失败"})
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var code, name, status, createdAt string
		var intro, majorName sql.NullString
		var price int64
		var unlocked bool
		if err := rows.Scan(&code, &name, &intro, &price, &majorName, &status, &unlocked, &createdAt); err != nil {
			c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "收藏查询失败"})
			return
		}
		items = append(items, gin.H{
			"course_code": code, "name": name, "intro": nullableString(intro),
			"price_cents": price, "major_name": nullableString(majorName),
			"course_status": status, "unlocked": unlocked || (price == 0 && status == "published"),
			"created_at": createdAt,
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"items": items}})
}
