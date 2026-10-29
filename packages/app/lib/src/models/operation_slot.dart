/// 运营位（`GET /public/operations`）。
class OperationSlot {
  const OperationSlot({
    required this.id,
    required this.position,
    required this.title,
    this.courseCode = '',
    this.imageUrl = '',
    this.linkUrl = '',
  });

  final int id;
  final String position;
  final String title;
  final String courseCode;
  final String imageUrl;
  final String linkUrl;

  factory OperationSlot.fromJson(Map<String, dynamic> json) => OperationSlot(
        id: (json['id'] as num?)?.toInt() ?? 0,
        position: json['position'] as String? ?? '',
        title: json['title'] as String? ?? '',
        courseCode: json['course_code'] as String? ?? '',
        imageUrl: json['image_url'] as String? ?? '',
        linkUrl: json['link_url'] as String? ?? '',
      );
}
