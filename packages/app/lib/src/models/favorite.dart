/// 我的收藏（`GET /me/favorites`）。
class FavoriteCourse {
  const FavoriteCourse({
    required this.courseCode,
    required this.name,
    this.intro,
    required this.priceCents,
    this.majorName,
    required this.courseStatus,
    required this.unlocked,
    required this.createdAt,
  });

  final String courseCode;
  final String name;
  final String? intro;
  final int priceCents;
  final String? majorName;
  final String courseStatus;
  final bool unlocked;
  final String createdAt;

  bool get offline => courseStatus != 'published';
  String get priceLabel => '¥${(priceCents / 100).toStringAsFixed(2)}';

  factory FavoriteCourse.fromJson(Map<String, dynamic> json) => FavoriteCourse(
        courseCode: json['course_code'] as String? ?? '',
        name: json['name'] as String? ?? '',
        intro: json['intro'] as String?,
        priceCents: (json['price_cents'] as num?)?.toInt() ?? 0,
        majorName: json['major_name'] as String?,
        courseStatus: json['course_status'] as String? ?? '',
        unlocked: json['unlocked'] as bool? ?? false,
        createdAt: json['created_at'] as String? ?? '',
      );
}
