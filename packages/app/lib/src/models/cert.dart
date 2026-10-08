/// 我的证书（对应 `GET /me/certs` 返回项）。
class Cert {
  const Cert({
    required this.certNo,
    required this.courseCode,
    required this.courseName,
    required this.level,
    required this.score,
    required this.band,
    required this.status,
    required this.issuedAt,
    required this.expiresAt,
  });

  final String certNo;
  final String courseCode;
  final String courseName;
  final String level;
  final double score;
  final String band;
  final String status;
  final String issuedAt;
  final String expiresAt;

  factory Cert.fromJson(Map<String, dynamic> json) {
    return Cert(
      certNo: json['cert_no'] as String,
      courseCode: json['course_code'] as String,
      courseName: json['course_name'] as String,
      level: json['level'] as String,
      score: (json['score'] as num?)?.toDouble() ?? 0,
      band: json['band'] as String,
      status: json['status'] as String,
      issuedAt: json['issued_at'] as String,
      expiresAt: json['expires_at'] as String,
    );
  }
}
