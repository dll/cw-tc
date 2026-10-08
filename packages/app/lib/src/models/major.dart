/// 专业分类（对应 `GET /api/v1/majors` 返回项）。
class Major {
  const Major({required this.id, required this.name});

  final int id;
  final String name;

  factory Major.fromJson(Map<String, dynamic> json) {
    return Major(
      id: (json['id'] as num).toInt(),
      name: json['name'] as String,
    );
  }
}
