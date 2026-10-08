/// 证书验真结果（对应 `GET /verify/:certNo` 返回）。
class VerifyResult {
  const VerifyResult({
    required this.certNo,
    required this.state,
    required this.course,
    required this.level,
    required this.name,
    required this.issuedAt,
    required this.expiresAt,
    this.revokedReason,
  });

  final String certNo;
  final String state; // valid / expired / revoked
  final String course;
  final String level;
  final String name; // 脱敏
  final String issuedAt;
  final String expiresAt;
  final String? revokedReason;

  bool get isValid => state == 'valid';

  factory VerifyResult.fromJson(Map<String, dynamic> json) {
    return VerifyResult(
      certNo: json['cert_no'] as String,
      state: json['state'] as String,
      course: json['course'] as String,
      level: json['level'] as String,
      name: json['name'] as String,
      issuedAt: json['issued_at'] as String,
      expiresAt: json['expires_at'] as String,
      revokedReason: json['revoked_reason'] as String?,
    );
  }
}
