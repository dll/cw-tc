/// 我的证书（对应 `GET /me/certs` 列表项）。
class Cert {
  const Cert({
    required this.certNo,
    required this.courseCode,
    required this.courseName,
    required this.level,
    required this.score,
    required this.band,
    required this.status,
    required this.displayStatus,
    required this.canDownload,
    this.revokedReason,
    required this.issuedAt,
    required this.expiresAt,
  });

  final String certNo;
  final String courseCode;
  final String courseName;
  final String level;
  final double score;
  final String band;
  final String status; // valid / revoked（库内原始状态）
  final String displayStatus; // valid / expiring / expired / revoked
  final bool canDownload;
  final String? revokedReason;
  final String issuedAt;
  final String expiresAt;

  bool get revoked => displayStatus == 'revoked';

  String get statusLabel {
    switch (displayStatus) {
      case 'expiring':
        return '即将到期';
      case 'expired':
        return '已过期';
      case 'revoked':
        return '已撤销';
      default:
        return '有效';
    }
  }

  String get revokedReasonLabel {
    switch (revokedReason) {
      case 'order_refunded':
        return '订单已退款';
      case 'admin':
        return '管理员撤销';
      default:
        return revokedReason ?? '';
    }
  }

  factory Cert.fromJson(Map<String, dynamic> json) {
    final status = json['status'] as String? ?? '';
    return Cert(
      certNo: json['cert_no'] as String? ?? '',
      courseCode: json['course_code'] as String? ?? '',
      courseName: json['course_name'] as String? ?? '',
      level: json['level'] as String? ?? '',
      score: (json['score'] as num?)?.toDouble() ?? 0,
      band: json['band'] as String? ?? '',
      status: status,
      displayStatus: json['display_status'] as String? ?? status,
      canDownload: json['can_download'] as bool? ?? status != 'revoked',
      revokedReason: json['revoked_reason'] as String?,
      issuedAt: json['issued_at'] as String? ?? '',
      expiresAt: json['expires_at'] as String? ?? '',
    );
  }
}
