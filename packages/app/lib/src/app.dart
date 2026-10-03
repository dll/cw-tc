import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'pages/course_square_page.dart';
import 'pages/home_page.dart';
import 'pages/placeholder_page.dart';

/// 第三课堂学生端（Android / Web 同一份代码，PRD v2 §10.3）。
///
/// M0 阶段只搭骨架与路由；各页面为占位实现，M1 起按 PRD §7 逐模块落地。
class CwTcApp extends StatelessWidget {
  const CwTcApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      title: '第三课堂',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorSchemeSeed: const Color(0xFF185FA5),
        useMaterial3: true,
      ),
      routerConfig: _router,
    );
  }
}

final GoRouter _router = GoRouter(
  routes: <RouteBase>[
    GoRoute(path: '/', builder: (_, __) => const HomePage()),
    GoRoute(path: '/courses', builder: (_, __) => const CourseSquarePage()),

    // M1 起的占位路由：先把信息架构（PRD v2 §8.1）固定下来，
    // 避免后续各自命名导致路径漂移。
    GoRoute(
      path: '/learn/:code/:no',
      builder: (_, state) => PlaceholderPage(
        title: '学习页',
        detail: '课程 ${state.pathParameters['code']} · 第 ${state.pathParameters['no']} 讲',
      ),
    ),
    GoRoute(
      path: '/exam/:code/:level',
      builder: (_, state) => PlaceholderPage(
        title: '测评页',
        detail: '课程 ${state.pathParameters['code']} · ${state.pathParameters['level']}',
      ),
    ),
    GoRoute(
      path: '/verify',
      builder: (_, __) => const PlaceholderPage(title: '证书验真', detail: '公开页，无需登录'),
    ),
    GoRoute(
      path: '/me',
      builder: (_, __) => const PlaceholderPage(title: '个人中心', detail: '我的课程 / 订单 / 成绩 / 证书'),
    ),
  ],
);
