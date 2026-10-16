/// 课程广场列表项（对应 `GET /api/v1/courses` 返回的公开字段）。
class Course {
  const Course({
    required this.courseCode,
    required this.name,
    this.intro,
    required this.priceCents,
    this.originalPriceCents,
    this.majorId,
    this.majorName,
    this.line,
    this.lectureCount = 0,
    this.freePreviewLectures = 0,
  });

  final String courseCode;
  final String name;
  final String? intro;
  final int priceCents;
  final int? originalPriceCents;
  final int? majorId;
  final String? majorName;
  final String? line;
  final int lectureCount;
  final int freePreviewLectures;

  factory Course.fromJson(Map<String, dynamic> json) {
    return Course(
      courseCode: json['course_code'] as String,
      name: json['name'] as String,
      intro: json['intro'] as String?,
      priceCents: (json['price_cents'] as num?)?.toInt() ?? 0,
      originalPriceCents: (json['original_price_cents'] as num?)?.toInt(),
      majorId: (json['major_id'] as num?)?.toInt(),
      majorName: json['major_name'] as String?,
      line: json['line'] as String?,
      lectureCount: (json['lecture_count'] as num?)?.toInt() ?? 0,
      freePreviewLectures: (json['free_preview_lectures'] as num?)?.toInt() ?? 0,
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

  /// 划线原价（无优惠或与现价相同时返回 null）。
  String? get originalPriceLabel {
    final original = originalPriceCents;
    if (original == null || original <= priceCents) return null;
    final yuan = original / 100;
    return yuan == yuan.roundToDouble()
        ? '¥${yuan.toInt()}'
        : '¥${yuan.toStringAsFixed(2)}';
  }

  /// 讲次与试学信息，用于选课时快速判断（无讲次时返回 null）。
  String? get lectureSummary {
    if (lectureCount <= 0) return null;
    final preview = freePreviewLectures > 0 ? ' · 可试学 $freePreviewLectures 讲' : '';
    return '共 $lectureCount 讲$preview';
  }
}

/// 课程广场分页结果（对应 `GET /api/v1/courses` 的分页信封 `{items,total,page,page_size}`）。
class CoursePage {
  const CoursePage({
    required this.items,
    required this.total,
    required this.page,
    required this.pageSize,
  });

  final List<Course> items;
  final int total;
  final int page;
  final int pageSize;

  factory CoursePage.fromJson(Map<String, dynamic>? data) {
    final rawItems = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return CoursePage(
      items: rawItems
          .map((e) => Course.fromJson(e as Map<String, dynamic>))
          .toList(),
      total: (data?['total'] as num?)?.toInt() ?? 0,
      page: (data?['page'] as num?)?.toInt() ?? 1,
      pageSize: (data?['page_size'] as num?)?.toInt() ?? 20,
    );
  }

  /// 是否还有下一页（用于无限滚动）。
  bool get hasMore => page * pageSize < total;
}
