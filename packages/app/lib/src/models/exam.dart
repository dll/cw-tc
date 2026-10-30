/// 测评会话（开考返回：session_id + 试卷）。
class ExamSession {
  const ExamSession({required this.sessionId, required this.paper});

  final int sessionId;
  final List<ExamQuestion> paper;
}

/// 测评题目（对应 `POST /exams` 返回的 paper 项，不含答案）。
class ExamQuestion {
  const ExamQuestion({
    required this.qno,
    required this.type,
    required this.stem,
    required this.score,
    this.options,
  });

  final String qno;
  final String type; // single / judge / ...
  final String stem;
  final double score;
  final List<String>? options;

  factory ExamQuestion.fromJson(Map<String, dynamic> json) {
    final rawOptions = json['options'] as List<dynamic>?;
    return ExamQuestion(
      qno: json['qno'] as String,
      type: json['type'] as String,
      stem: json['stem'] as String,
      score: (json['score'] as num?)?.toDouble() ?? 0,
      options: rawOptions?.map((e) => e.toString()).toList(),
    );
  }
}

/// 交卷结果（对应 `POST /exams/:id/submit` 返回）。
class ExamResult {
  const ExamResult({
    required this.autoScore,
    required this.totalScore,
    required this.isPass,
    required this.band,
    this.certNo,
  });

  final double autoScore;
  final double totalScore;
  final bool isPass;
  final String band;
  final String? certNo;

  factory ExamResult.fromJson(Map<String, dynamic> json) {
    return ExamResult(
      autoScore: (json['auto_score'] as num?)?.toDouble() ?? 0,
      totalScore: (json['total_score'] as num?)?.toDouble() ?? 0,
      isPass: json['is_pass'] as bool? ?? false,
      band: json['band'] as String? ?? '',
      certNo: json['cert_no'] as String?,
    );
  }
}

/// 未完成测评（断线 / 换设备恢复）。
class OngoingExam {
  const OngoingExam({
    required this.sessionId,
    required this.level,
    required this.paper,
    required this.answers,
    required this.remainingSecs,
    required this.expired,
  });

  final int sessionId;
  final String level;
  final List<ExamQuestion> paper;
  final Map<String, String> answers;
  final int remainingSecs;
  final bool expired;

  factory OngoingExam.fromJson(Map<String, dynamic> json) {
    final rawPaper = (json['paper'] as List<dynamic>?) ?? const <dynamic>[];
    final rawAnswers = (json['answers'] as Map<String, dynamic>?) ?? <String, dynamic>{};
    return OngoingExam(
      sessionId: (json['session_id'] as num?)?.toInt() ?? 0,
      level: json['level'] as String? ?? '',
      paper: rawPaper
          .map((e) => ExamQuestion.fromJson(e as Map<String, dynamic>))
          .toList(),
      answers: rawAnswers.map((k, v) => MapEntry(k, v.toString())),
      remainingSecs: (json['remaining_secs'] as num?)?.toInt() ?? 0,
      expired: json['expired'] as bool? ?? false,
    );
  }
}