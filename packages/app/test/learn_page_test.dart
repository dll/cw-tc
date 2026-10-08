import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/lecture_resource.dart';
import 'package:cw_tc_app/src/pages/learn_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// 假客户端：覆盖资源取数与文本拉取，不发真实网络请求（避免 pending timer）。
class _FakeApiClient extends ApiClient {
  @override
  Future<List<LectureResource>> lectureResources(String courseCode, int no) async {
    return const <LectureResource>[
      LectureResource(
        kind: 'markdown',
        title: '讲义',
        storageKey: 'AI化教学创新03-CF/01_你好世界/README.md',
        sizeBytes: 2242,
        expiresAt: 0,
        signature: 's',
      ),
      LectureResource(
        kind: 'code',
        title: '代码',
        storageKey: 'AI化教学创新03-CF/01_你好世界/main.c',
        sizeBytes: 100,
        expiresAt: 0,
        signature: 's',
      ),
    ];
  }

  @override
  Future<String> fetchResourceText(LectureResource r) async => '# 讲义内容';
}

void main() {
  testWidgets('学习页展示资源清单（文件名 + 类型 + 大小）', (WidgetTester tester) async {
    final api = _FakeApiClient();
    await tester.pumpWidget(
      MaterialApp(home: LearnPage(courseCode: 'C1', no: 1, api: api)),
    );
    await tester.pumpAndSettle();

    expect(find.text('README.md'), findsOneWidget);
    expect(find.text('讲义 · 2.2 KB'), findsOneWidget);
    expect(find.text('main.c'), findsOneWidget);
  });

  testWidgets('点击文本资源进入阅读页', (WidgetTester tester) async {
    final api = _FakeApiClient();
    await tester.pumpWidget(
      MaterialApp(home: LearnPage(courseCode: 'C1', no: 1, api: api)),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('README.md'));
    await tester.pumpAndSettle();

    expect(find.text('# 讲义内容'), findsOneWidget);
  });
}
