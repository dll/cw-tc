/// 我的课程（`GET /me/courses`）。
class MyCourse {
  const MyCourse({
    required this.courseCode,
    required this.name,
    this.intro,
    this.majorName,
    required this.lectureCount,
    required this.completedLectures,
    required this.percent,
    required this.lastLectureNo,
    required this.status,
  });

  final String courseCode;
  final String name;
  final String? intro;
  final String? majorName;
  final int lectureCount;
  final int completedLectures;
  final int percent;
  final int lastLectureNo;
  final String status; // learning / completed

  bool get completed => status == 'completed';

  factory MyCourse.fromJson(Map<String, dynamic> json) => MyCourse(
        courseCode: json['course_code'] as String? ?? '',
        name: json['name'] as String? ?? '',
        intro: json['intro'] as String?,
        majorName: json['major_name'] as String?,
        lectureCount: (json['lecture_count'] as num?)?.toInt() ?? 0,
        completedLectures: (json['completed_lectures'] as num?)?.toInt() ?? 0,
        percent: (json['percent'] as num?)?.toInt() ?? 0,
        lastLectureNo: (json['last_lecture_no'] as num?)?.toInt() ?? 1,
        status: json['status'] as String? ?? 'learning',
      );
}
