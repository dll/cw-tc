/// 我的成绩（`GET /me/scores`，PRD §7.13 L-3）。
class Score {
  const Score({
    required this.resultId,
    required this.courseCode,
    required this.courseName,
    required this.level,
    required this.autoScore,
    required this.manualScore,
    required this.totalScore,
    required this.band,
    required this.isPass,
    required this.gradedAt,
    required this.examAt,
    required this.certNo,
  });

  final int resultId;
  final String courseCode;
  final String courseName;
  final String level;
  final double autoScore;
  final double manualScore;
  final double totalScore;
  final String band;
  final bool isPass;
  final String gradedAt;
  final String examAt;
  final String certNo;

  String get levelLabel {
    switch (level) {
      case 'L1':
        return '初级';
      case 'L2':
        return '中级';
      case 'L3':
        return '高级';
      default:
        return level;
    }
  }

  String get bandLabel {
    switch (band) {
      case 'pass':
        return '合格';
      case 'good':
        return '良好';
      case 'excellent':
        return '优秀';
      case 'fail':
        return '未通过';
      default:
        return band.isEmpty ? '—' : band;
    }
  }

  factory Score.fromJson(Map<String, dynamic> json) => Score(
        resultId: (json['result_id'] as num?)?.toInt() ?? 0,
        courseCode: json['course_code'] as String? ?? '',
        courseName: json['course_name'] as String? ?? '',
        level: json['level'] as String? ?? '',
        autoScore: (json['auto_score'] as num?)?.toDouble() ?? 0,
        manualScore: (json['manual_score'] as num?)?.toDouble() ?? 0,
        totalScore: (json['total_score'] as num?)?.toDouble() ?? 0,
        band: json['band'] as String? ?? '',
        isPass: json['is_pass'] as bool? ?? false,
        gradedAt: json['graded_at'] as String? ?? '',
        examAt: json['exam_at'] as String? ?? '',
        certNo: json['cert_no'] as String? ?? '',
      );
}
