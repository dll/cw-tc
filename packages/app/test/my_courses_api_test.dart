import 'dart:convert';
import 'dart:typed_data';

import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

class _RespondingAdapter implements HttpClientAdapter {
  @override
  void close({bool force = false}) {}

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString(
        jsonEncode(<String, dynamic>{
          'code': 0,
          'data': {
            'items': [
              {
                'course_code': 'C1',
                'name': '课程一',
                'major_id': 2,
                'major_name': '计算机',
                'unlocked': true,
                'status': 'learning',
                'lecture_count': 8,
                'learned_lectures': 2,
                'progress_percent': 25,
                'last_lecture_no': 3,
                'last_learned_at': '2026-10-01T12:00:00Z',
              }
            ],
          },
        }),
        200,
        headers: {
          'content-type': ['application/json'],
        });
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('my courses parses contract and progress uses PUT boolean body',
      () async {
    FlutterSecureStorage.setMockInitialValues(
        <String, String>{'cw_tc_session': 'test-token'});
    final dio = Dio();
    final requests = <RequestOptions>[];
    dio.interceptors.add(InterceptorsWrapper(onRequest: (o, h) {
      requests.add(o);
      h.next(o);
    }));
    dio.httpClientAdapter = _RespondingAdapter();
    final api = ApiClient(dio: dio);

    final items = await api.myCourses();
    expect(items.single.courseCode, 'C1');
    expect(items.single.majorId, 2);
    expect(items.single.majorName, '计算机');
    expect(items.single.unlocked, isTrue);
    expect(items.single.status, 'learning');
    expect(items.single.lectureCount, 8);
    expect(items.single.learnedLectures, 2);
    expect(items.single.progressPercent, 25);
    expect(items.single.lastLectureNo, 3);
    expect(items.single.lastLearnedAt, DateTime.parse('2026-10-01T12:00:00Z'));

    await api.reportCourseProgress('C1', lectureNo: 3, completed: false);
    await api.reportCourseProgress('C1', lectureNo: 3, completed: true);

    expect(requests.first.path, '/api/v1/me/courses');
    expect(requests.first.method, 'GET');
    expect(requests[1].path, '/api/v1/me/courses/C1/progress');
    expect(requests[1].method, 'PUT');
    expect(requests[1].data,
        <String, dynamic>{'lecture_no': 3, 'completed': false});
    expect(requests[2].data,
        <String, dynamic>{'lecture_no': 3, 'completed': true});
    expect(requests.first.headers['Authorization'], 'Bearer test-token');
  });
}
