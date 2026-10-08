import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/app.dart';
import 'package:cw_tc_app/src/models/course.dart';
import 'package:cw_tc_app/src/pages/course_square_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// 测试用假客户端：不发真实网络请求（避免 pending timer）。
class _FakeApiClient extends ApiClient {
  _FakeApiClient(this._courses);

  final List<Course> _courses;

  @override
  Future<List<Course>> courses({int? majorId}) async => _courses;
}

void main() {
  testWidgets('首页渲染标题与入口按钮', (WidgetTester tester) async {
    await tester.pumpWidget(const CwTcApp());
    await tester.pumpAndSettle();

    expect(find.text('第三课堂'), findsWidgets);
    expect(find.text('进入课程广场'), findsOneWidget);
    expect(find.text('证书验真'), findsOneWidget);
  });

  testWidgets('课程广场展示课程列表', (WidgetTester tester) async {
    final api = _FakeApiClient(<Course>[
      const Course(courseCode: 'C1', name: '样例课程', priceCents: 100),
    ]);
    await tester.pumpWidget(MaterialApp(home: CourseSquarePage(api: api)));
    await tester.pumpAndSettle();

    expect(find.text('样例课程'), findsOneWidget);
    expect(find.text('¥1'), findsOneWidget);
  });

  testWidgets('课程广场空态', (WidgetTester tester) async {
    final api = _FakeApiClient(const <Course>[]);
    await tester.pumpWidget(MaterialApp(home: CourseSquarePage(api: api)));
    await tester.pumpAndSettle();

    expect(find.text('暂无已上架课程'), findsOneWidget);
  });
}
