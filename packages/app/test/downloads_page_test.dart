import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/download_grant.dart';
import 'package:cw_tc_app/src/pages/downloads_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _DownloadsApi extends ApiClient {
  _DownloadsApi(this.items);
  List<DownloadGrant> items;
  int? deletedLecture;
  String? deletedCourse;

  @override
  Future<List<DownloadGrant>> downloads() async => items;

  @override
  Future<void> deleteDownload(int lectureNo, {String? courseCode}) async {
    deletedLecture = lectureNo;
    deletedCourse = courseCode;
    items = items.where((item) => item.lectureNo != lectureNo).toList();
  }
}

void main() {
  testWidgets('shows offline grants and deletes one', (tester) async {
    final api = _DownloadsApi(<DownloadGrant>[
      const DownloadGrant(
        id: 1,
        courseCode: 'C1',
        courseName: '样例课程',
        lectureNo: 1,
        lectureTitle: '第一讲',
        expiresAt: '2026-12-01T00:00:00Z',
        status: 'valid',
        remainingDays: 30,
      ),
    ]);
    await tester.pumpWidget(MaterialApp(home: DownloadsPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.textContaining('样例课程'), findsOneWidget);
    expect(find.textContaining('剩余 30 天'), findsOneWidget);
    await tester.tap(find.byIcon(Icons.delete_outline));
    await tester.pumpAndSettle();
    expect(api.deletedLecture, 1);
    expect(api.deletedCourse, 'C1');
  });
}
