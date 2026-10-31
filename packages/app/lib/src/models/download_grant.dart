/// 离线下载授权，`GET /me/downloads`（PRD §7.4 B-11）。
class DownloadGrant {
  const DownloadGrant({
    required this.id,
    required this.courseCode,
    required this.courseName,
    required this.lectureNo,
    required this.lectureTitle,
    required this.expiresAt,
    required this.status,
    required this.remainingDays,
    this.revokedReason,
  });

  final int id;
  final String courseCode;
  final String courseName;
  final int lectureNo;
  final String lectureTitle;
  final String expiresAt;
  final String status;
  final int remainingDays;
  final String? revokedReason;

  bool get usable => status == 'valid' && remainingDays > 0;
  String get statusLabel {
    switch (status) {
      case 'valid':
        return '有效（剩余 $remainingDays 天）';
      case 'expired':
        return '已过期';
      case 'revoked':
        return revokedReason == 'order_refunded' ? '已退款失效' : '已撤销';
      default:
        return status;
    }
  }

  factory DownloadGrant.fromJson(Map<String, dynamic> json) => DownloadGrant(
        id: (json['id'] as num?)?.toInt() ?? 0,
        courseCode: json['course_code'] as String? ?? '',
        courseName: json['course_name'] as String? ?? '',
        lectureNo: (json['lecture_no'] as num?)?.toInt() ?? 0,
        lectureTitle: json['lecture_title'] as String? ?? '',
        expiresAt: json['expires_at'] as String? ?? '',
        status: json['status'] as String? ?? '',
        remainingDays: (json['remaining_days'] as num?)?.toInt() ?? 0,
        revokedReason: json['revoked_reason'] as String?,
      );
}
