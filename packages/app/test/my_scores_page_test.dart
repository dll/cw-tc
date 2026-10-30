import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/score.dart';
import 'package:cw_tc_app/src/pages/my_scores_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _ScoresApi extends ApiClient {
  _ScoresApi(this.items);

  final List<Score> items;
  bool fail = false;

  @override
  Future<List<Score>> myScores() async {
    if (fail) throw Exception('offline');
    return items;
  }
}

Score _score(int id, String course, double total, bool pass, String band) =>
    Score(
      resultId: id,
      courseCode: 'C1',
      courseName: course,
      level: 'L1',
      autoScore: total,
      manualScore: 0,
      totalScore: total,
      band: band,
      isPass: pass,
      gradedAt: '2026-10-01T10:31:00Z',
      examAt: '2026-10-01T10:30:00Z',
      certNo: pass ? 'CERT-$id' : '',
    );

void main() {
  testWidgets('groups attempts and shows failures', (tester) async {
    final api = _ScoresApi(<Score>[
      _score(1, '课程一', 88, true, 'pass'),
      _score(2, '课程一', 40, false, 'fail'),
    ]);
    await tester.pumpWidget(MaterialApp(home: MyScoresPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('课程一'), findsOneWidget);
    expect(find.textContaining('88.0 分'), findsOneWidget);
    expect(find.textContaining('40.0 分'), findsOneWidget);
    expect(find.textContaining('未达标'), findsOneWidget);
    expect(find.textContaining('CERT-1'), findsOneWidget);
  });

  testWidgets('shows empty state', (tester) async {
    await tester.pumpWidget(
        MaterialApp(home: MyScoresPage(api: _ScoresApi(<Score>[]))));
    await tester.pumpAndSettle();
    expect(find.text('暂无成绩，完成测评后可查看'), findsOneWidget);
  });

  testWidgets('shows error and retries', (tester) async {
    final api = _ScoresApi(<Score>[_score(1, '课程一', 88, true, 'pass')])
      ..fail = true;
    await tester.pumpWidget(MaterialApp(home: MyScoresPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('成绩加载失败，请重试'), findsOneWidget);
    api.fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('课程一'), findsOneWidget);
  });
}
