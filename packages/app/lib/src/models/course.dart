/// 课程广场列表项（对应 `GET /api/v1/courses` 返回的公开字段）。
class Course {
  const Course({
    required this.courseCode,
    required this.name,
    this.intro,
    required this.priceCents,
    this.originalPriceCents,
    this.majorId,
  });

  final String courseCode;
  final String name;
  final String? intro;
  final int priceCents;
  final int? originalPriceCents;
  final int? majorId;

  factory Course.fromJson(Map<String, dynamic> json) {
    return Course(
      courseCode: json['course_code'] as String,
      name: json['name'] as String,
      intro: json['intro'] as String?,
      priceCents: (json['price_cents'] as num?)?.toInt() ?? 0,
      originalPriceCents: (json['original_price_cents'] as num?)?.toInt(),
      majorId: (json['major_id'] as num?)?.toInt(),
    );
  }

  /// 价格展示：分 → 元（免费课显示「免费」）。
  String get priceLabel {
    if (priceCents <= 0) return '免费';
    final yuan = priceCents / 100;
    return yuan == yuan.roundToDouble()
        ? '¥${yuan.toInt()}'
        : '¥${yuan.toStringAsFixed(2)}';
  }
}
