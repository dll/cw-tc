class Note {
  const Note({required this.id, required this.courseCode, this.lectureNo, required this.title, required this.body, required this.source, required this.updatedAt});
  final int id; final String courseCode; final int? lectureNo; final String title; final String body; final String source; final String updatedAt;
  factory Note.fromJson(Map<String, dynamic> j) => Note(id:(j['id'] as num?)?.toInt()??0, courseCode:j['course_code'] as String? ??'', lectureNo:(j['lecture_no'] as num?)?.toInt(), title:j['title'] as String? ??'', body:j['body'] as String? ??'', source:j['source'] as String? ??'', updatedAt:j['updated_at'] as String? ??'');
}
class Suggestion {
  const Suggestion({required this.id, this.courseCode, required this.body, required this.status, this.reply, required this.createdAt});
  final int id; final String? courseCode; final String body; final String status; final String? reply; final String createdAt;
  factory Suggestion.fromJson(Map<String, dynamic> j) => Suggestion(id:(j['id'] as num?)?.toInt()??0, courseCode:j['course_code'] as String?, body:j['body'] as String? ??'', status:j['status'] as String? ??'', reply:j['reply'] as String?, createdAt:j['created_at'] as String? ??'');
}
