/// 讲次（对应 `GET /api/v1/courses/:code/lectures` 返回项）。
class Lecture {
  const Lecture({required this.no, required this.title, required this.isPreview});

  final int no;
  final String title;
  final bool isPreview;

  factory Lecture.fromJson(Map<String, dynamic> json) {
    return Lecture(
      no: (json['no'] as num).toInt(),
      title: json['title'] as String,
      isPreview: json['is_preview'] as bool? ?? false,
    );
  }
}
