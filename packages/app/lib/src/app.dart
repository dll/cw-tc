import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'pages/course_square_page.dart';
import 'pages/home_page.dart';
import 'pages/me_page.dart';
import 'pages/placeholder_page.dart';
import 'shell.dart';

/// 第三课堂学生端（Android / Web 同一份代码，PRD v2 §10.3）。
///
/// M0 阶段搭骨架与信息架构：底部导航（首页/课程/我的）+ 各业务整页路由；
/// 各页为占位实现，M1 起按 PRD §7 逐模块落地。
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
    // —— 带底部导航的三个顶级分支 ——
    StatefulShellRoute.indexedStack(
      builder: (BuildContext context, GoRouterState state, StatefulNavigationShell shell) =>
          AppShell(navigationShell: shell),
      branches: <StatefulShellBranch>[
        StatefulShellBranch(routes: <RouteBase>[
          GoRoute(path: '/', builder: (_, __) => const HomePage()),
        ]),
        StatefulShellBranch(routes: <RouteBase>[
          GoRoute(path: '/courses', builder: (_, __) => const CourseSquarePage()),
        ]),
        StatefulShellBranch(routes: <RouteBase>[
          GoRoute(path: '/me', builder: (_, __) => const MePage()),
        ]),
      ],
    ),

    // —— 整页业务路由（不进底部导航；信息架构按 PRD v2 §8.1 固定）——
    GoRoute(
      path: '/learn/:code/:no',
      builder: (_, GoRouterState s) => PlaceholderPage(
        title: '学习页',
        detail: '课程 ${s.pathParameters['code']} · 第 ${s.pathParameters['no']} 讲',
      ),
    ),
    GoRoute(
      path: '/exam/:code/:level',
      builder: (_, GoRouterState s) => PlaceholderPage(
        title: '测评页',
        detail: '课程 ${s.pathParameters['code']} · ${s.pathParameters['level']}',
      ),
    ),
    GoRoute(
      path: '/verify',
      builder: (_, __) => const PlaceholderPage(title: '证书验真', detail: '公开页，无需登录'),
    ),
  ],
);
