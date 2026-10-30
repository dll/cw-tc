import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/pages/me_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

/// 假客户端：固定登录态与手机号，不发真实请求（避免 pending timer）。
class _FakeApiClient extends ApiClient {
  _FakeApiClient(this._loggedIn, this._phone);

  final bool _loggedIn;
  final String? _phone;

  @override
  Future<bool> hasSession() async => _loggedIn;

  @override
  Future<String?> storedPhone() async => _phone;
}

void main() {
  testWidgets('my courses entry navigates to /me/courses', (tester) async {
    final router = GoRouter(initialLocation: '/me', routes: <RouteBase>[
      GoRoute(
          path: '/me',
          builder: (_, __) => MePage(api: _FakeApiClient(true, null))),
      GoRoute(
          path: '/me/courses',
          builder: (_, __) =>
              const Scaffold(body: Text('my-courses-destination'))),
    ]);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();
    await tester.tap(find.text('我的课程'));
    await tester.pumpAndSettle();
    expect(find.text('my-courses-destination'), findsOneWidget);
    router.dispose();
  });

  testWidgets('my orders entry navigates to /me/orders', (tester) async {
    final router = GoRouter(initialLocation: '/me', routes: <RouteBase>[
      GoRoute(
          path: '/me',
          builder: (_, __) => MePage(api: _FakeApiClient(true, null))),
      GoRoute(
          path: '/me/orders',
          builder: (_, __) =>
              const Scaffold(body: Text('my-orders-destination'))),
    ]);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();
    await tester.tap(find.text('我的订单'));
    await tester.pumpAndSettle();
    expect(find.text('my-orders-destination'), findsOneWidget);
    router.dispose();
  });

  testWidgets('未登录显示未登录', (WidgetTester tester) async {
    await tester.pumpWidget(
      MaterialApp(home: MePage(api: _FakeApiClient(false, null))),
    );
    await tester.pumpAndSettle();

    expect(find.text('未登录'), findsOneWidget);
    expect(find.text('手机号注册 / 登录'), findsOneWidget);
  });

  testWidgets('已登录显示脱敏手机号与退出', (WidgetTester tester) async {
    await tester.pumpWidget(
      MaterialApp(home: MePage(api: _FakeApiClient(true, '13812345678'))),
    );
    await tester.pumpAndSettle();

    expect(find.text('138****5678'), findsOneWidget);
    expect(find.text('已登录'), findsOneWidget);
    expect(find.text('退出登录'), findsOneWidget);
  });
}
