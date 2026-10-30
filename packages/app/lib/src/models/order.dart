/// 退款进度（`/me/orders` 内嵌，PRD §7.13 L-2）。
class OrderRefund {
  const OrderRefund({
    required this.refundNo,
    required this.amountCents,
    required this.status,
    required this.reason,
    this.failReason,
    required this.createdAt,
    this.finishedAt,
  });

  final String refundNo;
  final int amountCents;
  final String status; // processing / success / failed
  final String reason;
  final String? failReason;
  final String createdAt;
  final String? finishedAt;

  String get statusLabel {
    switch (status) {
      case 'processing':
        return '退款处理中';
      case 'success':
        return '已退款';
      case 'failed':
        return '退款失败';
      default:
        return status;
    }
  }

  factory OrderRefund.fromJson(Map<String, dynamic> json) => OrderRefund(
        refundNo: json['refund_no'] as String? ?? '',
        amountCents: (json['amount_cents'] as num?)?.toInt() ?? 0,
        status: json['status'] as String? ?? '',
        reason: json['reason'] as String? ?? '',
        failReason: json['fail_reason'] as String?,
        createdAt: json['created_at'] as String? ?? '',
        finishedAt: json['finished_at'] as String?,
      );
}

/// 我的订单（`GET /me/orders`）。
class MyOrder {
  const MyOrder({
    required this.orderNo,
    required this.courseCode,
    required this.courseName,
    required this.amountCents,
    required this.status,
    required this.channel,
    required this.createdAt,
    this.paidAt,
    required this.expiredAt,
    this.refundedAt,
    required this.canContinuePay,
    required this.canCancel,
    required this.canRefund,
    required this.canEnterCourse,
    required this.examTaken,
    required this.certIssued,
    required this.certRevoked,
    this.refundableUntil,
    this.refund,
  });

  final String orderNo;
  final String courseCode;
  final String courseName;
  final int amountCents;
  final String status; // pending / paid / closed / refunded
  final String channel;
  final String createdAt;
  final String? paidAt;
  final String expiredAt;
  final String? refundedAt;
  final bool canContinuePay;
  final bool canCancel;
  final bool canRefund;
  final bool canEnterCourse;
  final bool examTaken;
  final bool certIssued;
  final bool certRevoked;
  final String? refundableUntil;
  final OrderRefund? refund;

  bool get pending => status == 'pending';
  bool get paid => status == 'paid';
  bool get closed => status == 'closed';
  bool get refunded => status == 'refunded';

  String get statusLabel {
    switch (status) {
      case 'pending':
        return '待支付';
      case 'paid':
        return '已支付';
      case 'closed':
        return '已关闭';
      case 'refunded':
        return '已退款';
      default:
        return status;
    }
  }

  String get channelLabel {
    switch (channel) {
      case 'wechat':
        return '微信';
      case 'alipay':
        return '支付宝';
      default:
        return '未选择';
    }
  }

  factory MyOrder.fromJson(Map<String, dynamic> json) {
    final refund = json['refund'];
    return MyOrder(
      orderNo: json['order_no'] as String? ?? '',
      courseCode: json['course_code'] as String? ?? '',
      courseName: json['course_name'] as String? ?? '',
      amountCents: (json['amount_cents'] as num?)?.toInt() ?? 0,
      status: json['status'] as String? ?? '',
      channel: json['channel'] as String? ?? '',
      createdAt: json['created_at'] as String? ?? '',
      paidAt: json['paid_at'] as String?,
      expiredAt: json['expired_at'] as String? ?? '',
      refundedAt: json['refunded_at'] as String?,
      canContinuePay: json['can_continue_pay'] as bool? ?? false,
      canCancel: json['can_cancel'] as bool? ?? false,
      canRefund: json['can_refund'] as bool? ?? false,
      canEnterCourse: json['can_enter_course'] as bool? ?? false,
      examTaken: json['exam_taken'] as bool? ?? false,
      certIssued: json['cert_issued'] as bool? ?? false,
      certRevoked: json['cert_revoked'] as bool? ?? false,
      refundableUntil: json['refundable_until'] as String?,
      refund: refund is Map<String, dynamic>
          ? OrderRefund.fromJson(refund)
          : null,
    );
  }
}

/// 继续支付返回（支付宝二维码或 mock 渠道）。
class OrderPaymentIntent {
  const OrderPaymentIntent({
    required this.orderNo,
    required this.channel,
    required this.amountCents,
    this.qrCode,
    this.expiresAt,
  });

  final String orderNo;
  final String channel;
  final int amountCents;
  final String? qrCode;
  final String? expiresAt;

  factory OrderPaymentIntent.fromJson(Map<String, dynamic> json) =>
      OrderPaymentIntent(
        orderNo: json['order_no'] as String? ?? '',
        channel: json['channel'] as String? ?? '',
        amountCents: (json['amount_cents'] as num?)?.toInt() ?? 0,
        qrCode: json['qr_code'] as String?,
        expiresAt: json['expires_at'] as String?,
      );
}
