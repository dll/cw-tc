import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/pages/me_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

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
