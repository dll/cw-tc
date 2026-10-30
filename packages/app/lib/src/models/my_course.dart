/// 我的课程（`GET /me/courses`，PRD §7.13 L-1）。
class MyCourse {
  const MyCourse({
    required this.courseCode,
    required this.name,
    this.majorId,
    this.majorName,
    required this.unlocked,
    required this.status,
    required this.lectureCount,
    required this.learnedLectures,
    required this.progressPercent,
    this.lastLectureNo,
    this.lastLearnedAt,
  });

  final String courseCode;
  final String name;
  final int? majorId;
  final String? majorName;
  final bool unlocked;
  final String status; // locked / learning / completed
  final int lectureCount;
  final int learnedLectures;
  final int progressPercent;
  final int? lastLectureNo;
  final DateTime? lastLearnedAt;

  bool get completed => status == 'completed';
  bool get locked => status == 'locked' || !unlocked;

  factory MyCourse.fromJson(Map<String, dynamic> json) => MyCourse(
        courseCode: json['course_code'] as String? ?? '',
        name: json['name'] as String? ?? '',
        majorId: (json['major_id'] as num?)?.toInt(),
        majorName: json['major_name'] as String?,
        unlocked: json['unlocked'] as bool? ?? false,
        status: json['status'] as String? ?? 'locked',
        lectureCount: (json['lecture_count'] as num?)?.toInt() ?? 0,
        learnedLectures: (json['learned_lectures'] as num?)?.toInt() ?? 0,
        progressPercent: (json['progress_percent'] as num?)?.toInt() ?? 0,
        lastLectureNo: (json['last_lecture_no'] as num?)?.toInt(),
        lastLearnedAt:
            DateTime.tryParse(json['last_learned_at'] as String? ?? ''),
      );
}
