class AiCitation {
  const AiCitation({required this.lectureNo, required this.title, this.storageKey});

  final int lectureNo;
  final String title;
  final String? storageKey;

  factory AiCitation.fromJson(Map<String, dynamic> json) => AiCitation(
        lectureNo: (json['lecture_no'] as num?)?.toInt() ?? 0,
        title: json['title'] as String? ?? '',
        storageKey: json['storage_key'] as String?,
      );
}

class AiAnswer {
  const AiAnswer({
    required this.answer,
    required this.outcome,
    required this.citations,
    required this.model,
    required this.remaining,
  });

  final String answer;
  final String outcome;
  final List<AiCitation> citations;
  final String model;
  final int remaining;

  factory AiAnswer.fromJson(Map<String, dynamic> json) {
    final raw = (json['citations'] as List<dynamic>?) ?? const <dynamic>[];
    return AiAnswer(
      answer: json['answer'] as String? ?? '',
      outcome: json['outcome'] as String? ?? '',
      citations: raw
          .map((e) => AiCitation.fromJson(e as Map<String, dynamic>))
          .toList(),
      model: json['model'] as String? ?? '',
      remaining: (json['remaining'] as num?)?.toInt() ?? 0,
    );
  }
}
