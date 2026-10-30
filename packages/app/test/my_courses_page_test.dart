import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/my_course.dart';
import 'package:cw_tc_app/src/pages/my_courses_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _CoursesApi extends ApiClient {
  _CoursesApi(this.items);

  final List<MyCourse> items;
  int calls = 0;
  bool fail = false;

  @override
  Future<List<MyCourse>> myCourses() async {
    calls++;
    if (fail) throw Exception('offline');
    return items;
  }
}

MyCourse _course(
  String code,
  String name, {
  int major = 1,
  String majorName = '计算机',
  String status = 'learning',
  bool unlocked = true,
}) =>
    MyCourse(
      courseCode: code,
      name: name,
      majorId: major,
      majorName: majorName,
      unlocked: unlocked,
      status: status,
      lectureCount: 4,
      learnedLectures: 1,
      progressPercent: 25,
      lastLectureNo: 2,
    );

void main() {
  testWidgets('filters course status and major', (tester) async {
    final api = _CoursesApi(<MyCourse>[
      _course('A', '学习课程'),
      _course('B', '完成课程', major: 2, majorName: '数学', status: 'completed'),
      _course('C', '锁定课程', status: 'locked', unlocked: false),
    ]);
    await tester.pumpWidget(MaterialApp(home: MyCoursesPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('学习课程'), findsOneWidget);
    expect(find.text('完成课程'), findsOneWidget);

    await tester.tap(find.text('已完成'));
    await tester.pumpAndSettle();
    expect(find.text('完成课程'), findsOneWidget);
    expect(find.text('学习课程'), findsNothing);

    await tester.tap(find.text('全部'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('全部专业'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('数学').last);
    await tester.pumpAndSettle();
    expect(find.text('完成课程'), findsOneWidget);
    expect(find.text('学习课程'), findsNothing);
  });

  testWidgets('shows error and retries loading', (tester) async {
    final api = _CoursesApi(<MyCourse>[_course('A', '课程')])..fail = true;
    await tester.pumpWidget(MaterialApp(home: MyCoursesPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('课程加载失败，请重试'), findsOneWidget);

    api.fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('课程'), findsOneWidget);
    expect(api.calls, 2);
  });

  testWidgets('shows empty list', (tester) async {
    await tester.pumpWidget(
        MaterialApp(home: MyCoursesPage(api: _CoursesApi(<MyCourse>[]))));
    await tester.pumpAndSettle();
    expect(find.text('暂无符合条件的课程'), findsOneWidget);
  });

  testWidgets('resumes an unlocked course at its last lecture', (tester) async {
    final api = _CoursesApi(<MyCourse>[_course('A/B', '学习课程')]);
    final router = GoRouter(initialLocation: '/mine', routes: <RouteBase>[
      GoRoute(
          path: '/mine', builder: (_, __) => MyCoursesPage(api: api)),
      GoRoute(
          path: '/learn/:code/:no',
          builder: (_, state) => Scaffold(
              body: Text(
                  'learn:${state.pathParameters['code']}:${state.pathParameters['no']}'))),
      GoRoute(
          path: '/courses/:code',
          builder: (_, state) =>
              Scaffold(body: Text('detail:${state.pathParameters['code']}'))),
    ]);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();
    await tester.tap(find.text('学习课程'));
    await tester.pumpAndSettle();
    expect(find.text('learn:A/B:2'), findsOneWidget);
    router.dispose();
  });

  testWidgets('navigates locked course to detail', (tester) async {
    final api = _CoursesApi(
        <MyCourse>[_course('LOCK', '锁定课程', status: 'locked', unlocked: false)]);
    final router = GoRouter(initialLocation: '/mine', routes: <RouteBase>[
      GoRoute(
          path: '/mine', builder: (_, __) => MyCoursesPage(api: api)),
      GoRoute(
          path: '/courses/:code',
          builder: (_, state) =>
              Scaffold(body: Text('detail:${state.pathParameters['code']}'))),
    ]);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();
    await tester.tap(find.text('锁定课程'));
    await tester.pumpAndSettle();
    expect(find.text('detail:LOCK'), findsOneWidget);
    router.dispose();
  });
}
