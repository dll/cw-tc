package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// adminListReconcile GET /admin/reconcile —— 对账结果（差异必须为 0）。
func (s *Server) adminListReconcile(c *gin.Context) {
	limit, limitErr := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, offsetErr := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limitErr != nil || offsetErr != nil || limit < 1 || limit > 100 || offset < 0 {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "分页参数不合法"})
		return
	}
	query := `SELECT biz_date, channel, channel_count, channel_amount_cents, local_count, local_amount_cents,
	                 diff_count, diff_amount_cents, created_at
	          FROM reconcile_record`
	args := make([]any, 0, 3)
	if channel := strings.TrimSpace(c.Query("channel")); channel != "" {
		if channel != "wechat" && channel != "alipay" {
			c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "渠道只接受 wechat / alipay"})
			return
		}
		args = append(args, channel)
		query += ` WHERE channel = $1`
	}
	args = append(args, limit, offset)
	query += fmt.Sprintf(` ORDER BY biz_date DESC, channel LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	items, err := s.queryReconcile(c, query, args...)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "对账结果读取失败"})
		return
	}
	totalDiff := int64(0)
	for _, item := range items {
		if diff, ok := item["diff_amount_cents"].(int64); ok {
			if diff < 0 {
				diff = -diff
			}
			totalDiff += diff
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"items": items, "diff_abs_total_cents": totalDiff, "all_balanced": totalDiff == 0,
	}})
}

// adminReconcileDay GET /admin/reconcile/:date —— 按日对账明细（逐渠道）。
func (s *Server) adminReconcileDay(c *gin.Context) {
	date := strings.TrimSpace(c.Param("date"))
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "日期格式应为 YYYY-MM-DD"})
		return
	}
	items, err := s.queryReconcile(c,
		`SELECT biz_date, channel, channel_count, channel_amount_cents, local_count, local_amount_cents,
		        diff_count, diff_amount_cents, created_at
		 FROM reconcile_record WHERE biz_date = $1 ORDER BY channel`, date)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "对账明细读取失败"})
		return
	}
	balanced := true
	for _, item := range items {
		if item["diff_count"] != int64(0) || item["diff_amount_cents"] != int64(0) {
			balanced = false
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"biz_date": date, "items": items, "all_balanced": balanced,
	}})
}

// adminReconcileDiff GET /admin/reconcile/:date/diff —— 差异明细 CSV（差异 ≠ 0 时必看）。
func (s *Server) adminReconcileDiffCSV(c *gin.Context) {
	date := strings.TrimSpace(c.Param("date"))
	if _, err := time.Parse("2006-01-02", date); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 10001, Msg: "日期格式应为 YYYY-MM-DD"})
		return
	}
	rows, err := s.dbConn.QueryContext(c.Request.Context(),
		`SELECT biz_date, channel, channel_count, channel_amount_cents, local_count, local_amount_cents,
		        diff_count, diff_amount_cents
		 FROM reconcile_record
		 WHERE biz_date = $1 AND (diff_count <> 0 OR diff_amount_cents <> 0)
		 ORDER BY channel`, date)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10902, Msg: "差异明细读取失败"})
		return
	}
	defer rows.Close()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="reconcile-`+date+`-diff.csv"`)
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"日期", "渠道", "渠道笔数", "渠道金额(分)", "本地笔数", "本地金额(分)", "差异笔数", "差异金额(分)"})
	for rows.Next() {
		var bizDate, channel string
		var channelCount, channelAmount, localCount, localAmount, diffCount, diffAmount int64
		if err := rows.Scan(&bizDate, &channel, &channelCount, &channelAmount,
			&localCount, &localAmount, &diffCount, &diffAmount); err != nil {
			break
		}
		_ = writer.Write([]string{
			bizDate, channel, strconv.FormatInt(channelCount, 10), strconv.FormatInt(channelAmount, 10),
			strconv.FormatInt(localCount, 10), strconv.FormatInt(localAmount, 10),
			strconv.FormatInt(diffCount, 10), strconv.FormatInt(diffAmount, 10),
		})
	}
	writer.Flush()
}

// adminPaymentChannels GET /admin/payments/channels —— 只回"已配置 / 未配置"与商户号后四位，绝不返回值。
func (s *Server) adminPaymentChannels(c *gin.Context) {
	wechat := gin.H{"configured": false, "merchant_last4": nil}
	alipay := gin.H{"configured": false, "merchant_last4": nil}
	if s.cfg != nil {
		wechat["configured"] = s.cfg.WXPayEnabled()
		wechat["merchant_last4"] = last4(s.cfg.WXPayMchID)
		alipay["configured"] = s.cfg.AlipayEnabled()
		alipay["merchant_last4"] = last4(s.cfg.AlipayAppID)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{
		"items": []gin.H{
			{"channel": "wechat", "configured": wechat["configured"], "merchant_last4": wechat["merchant_last4"]},
			{"channel": "alipay", "configured": alipay["configured"], "merchant_last4": alipay["merchant_last4"]},
		},
		"note": "只回配置状态与商户号后四位，绝不返回密钥值",
	}})
}

func (s *Server) queryReconcile(c *gin.Context, query string, args ...any) ([]gin.H, error) {
	rows, err := s.dbConn.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var bizDate, channel, createdAt string
		var channelCount, channelAmount, localCount, localAmount, diffCount, diffAmount int64
		if err := rows.Scan(&bizDate, &channel, &channelCount, &channelAmount,
			&localCount, &localAmount, &diffCount, &diffAmount, &createdAt); err != nil {
			return nil, err
		}
		items = append(items, gin.H{
			"biz_date": bizDate, "channel": channel,
			"channel_count": channelCount, "channel_amount_cents": channelAmount,
			"local_count": localCount, "local_amount_cents": localAmount,
			"diff_count": diffCount, "diff_amount_cents": diffAmount,
			"balanced":   diffCount == 0 && diffAmount == 0,
			"created_at": createdAt,
		})
	}
	return items, rows.Err()
}

func last4(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) <= 4 {
		return value
	}
	return value[len(value)-4:]
}
