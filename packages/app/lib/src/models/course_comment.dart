class CourseComment {
  const CourseComment({
    required this.nickname,
    required this.rating,
    this.body,
    this.ratingContent,
    this.ratingLevel,
    this.ratingClarity,
    required this.completedLectures,
    required this.totalLectures,
    this.editedAt,
    required this.createdAt,
  });

  final String nickname;
  final int rating;
  final String? body;
  final int? ratingContent;
  final int? ratingLevel;
  final int? ratingClarity;
  final int completedLectures;
  final int totalLectures;
  final String? editedAt;
  final String createdAt;

  factory CourseComment.fromJson(Map<String, dynamic> json) => CourseComment(
        nickname: json['nickname'] as String? ?? '学员***',
        rating: (json['rating'] as num?)?.toInt() ?? 0,
        body: json['body'] as String?,
        ratingContent: (json['rating_content'] as num?)?.toInt(),
        ratingLevel: (json['rating_level'] as num?)?.toInt(),
        ratingClarity: (json['rating_clarity'] as num?)?.toInt(),
        completedLectures: (json['completed_lectures'] as num?)?.toInt() ?? 0,
        totalLectures: (json['total_lectures'] as num?)?.toInt() ?? 0,
        editedAt: json['edited_at'] as String?,
        createdAt: json['created_at'] as String? ?? '',
      );
}

class CourseCommentPage {
  const CourseCommentPage({
    required this.items,
    required this.count,
    required this.average,
    required this.threshold,
  });

  final List<CourseComment> items;
  final int count;
  final double? average;
  final int threshold;

  factory CourseCommentPage.fromJson(Map<String, dynamic> json) {
    final raw = (json['items'] as List<dynamic>?) ?? const <dynamic>[];
    return CourseCommentPage(
      items: raw
          .map((e) => CourseComment.fromJson(e as Map<String, dynamic>))
          .toList(),
      count: (json['count'] as num?)?.toInt() ?? 0,
      average: (json['average'] as num?)?.toDouble(),
      threshold: (json['display_threshold'] as num?)?.toInt() ?? 10,
    );
  }
}
