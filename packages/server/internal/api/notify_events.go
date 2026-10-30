package api

import (
	"context"
	"database/sql"
)

func (s *Server) pushNotice(ctx context.Context, studentID int64, kind, title, body, link string) {
	if s.dbConn == nil || studentID <= 0 || title == "" {
		return
	}
	_, _ = s.dbConn.ExecContext(ctx,
		`INSERT INTO notification(student_id,type,title,body,link)
		 SELECT $1,$2,$3,$4,$5
		 WHERE NOT EXISTS (SELECT 1 FROM notification WHERE student_id=$1 AND type=$2 AND link=$5 AND title=$3)`,
		studentID, kind, title, body, link)
}

func (s *Server) notifyOrderPaid(ctx context.Context, orderNo string) {
	var studentID int64
	var courseName string
	if err := s.dbConn.QueryRowContext(ctx,
		`SELECT student_id, course_name_snapshot FROM orders WHERE order_no=$1`, orderNo).Scan(&studentID, &courseName); err != nil {
		return
	}
	s.pushNotice(ctx, studentID, "order", "支付成功", "课程《"+courseName+"》已开通学习权益。", "/me/orders")
}

func (s *Server) notifyOrderRefunded(ctx context.Context, orderNo string) {
	var studentID int64
	var courseName string
	if err := s.dbConn.QueryRowContext(ctx,
		`SELECT student_id, course_name_snapshot FROM orders WHERE order_no=$1`, orderNo).Scan(&studentID, &courseName); err != nil {
		return
	}
	s.pushNotice(ctx, studentID, "order", "退款成功", "课程《"+courseName+"》退款已处理，权益与证书状态已同步。", "/me/orders")
}

func (s *Server) notifyExamResult(ctx context.Context, studentID int64, pass bool, certNo string) {
	if pass {
		s.pushNotice(ctx, studentID, "exam", "测评通过", "本次测评已通过，可在我的成绩查看详情。", "/me/scores")
		if certNo != "" {
			s.pushNotice(ctx, studentID, "cert", "证书已签发", "证书编号："+certNo, "/certs")
		}
		return
	}
	s.pushNotice(ctx, studentID, "exam", "测评成绩已出", "本次测评未达标，可查看成绩后按冷却规则重考。", "/me/scores")
}

var _ = sql.ErrNoRows
