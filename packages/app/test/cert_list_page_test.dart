import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/cert.dart';
import 'package:cw_tc_app/src/pages/cert_list_page.dart';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _CertsApi extends ApiClient {
  _CertsApi(this.items);

  final List<Cert> items;
  bool fail = false;

  @override
  Future<List<Cert>> myCerts() async {
    if (fail) throw Exception('offline');
    return items;
  }

  @override
  Future<String> certHtml(String certNo) async =>
      '<html><body>$certNo</body></html>';
}

Cert _cert(String no, String status,
        {String? revokedReason, bool canDownload = true}) =>
    Cert(
      certNo: no,
      courseCode: 'C1',
      courseName: '课程 $no',
      level: 'L1',
      score: 88,
      band: 'pass',
      status: status == 'revoked' ? 'revoked' : 'valid',
      displayStatus: status,
      canDownload: canDownload,
      revokedReason: revokedReason,
      issuedAt: '2026-01-01T00:00:00Z',
      expiresAt: '2029-01-01T00:00:00Z',
    );

void main() {
  testWidgets('shows all four certificate statuses', (tester) async {
    final api = _CertsApi(<Cert>[
      _cert('V1', 'valid'),
      _cert('E1', 'expiring'),
      _cert('X1', 'expired'),
      _cert('R1', 'revoked',
          revokedReason: 'order_refunded', canDownload: false),
    ]);
    await tester.pumpWidget(MaterialApp(home: CertListPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('有效'), findsWidgets);
    expect(find.text('即将到期'), findsWidgets);
    expect(find.text('已过期'), findsWidgets);
    expect(find.text('已撤销'), findsWidgets);
    await tester.drag(find.byType(ListView), const Offset(0, -500));
    await tester.pumpAndSettle();
    expect(find.text('撤销原因：订单已退款'), findsOneWidget);
    expect(find.text('不可下载'), findsOneWidget);
  });

  testWidgets('filters revoked certificates', (tester) async {
    final api = _CertsApi(<Cert>[
      _cert('V1', 'valid'),
      _cert('R1', 'revoked', revokedReason: 'admin', canDownload: false),
    ]);
    await tester.pumpWidget(MaterialApp(home: CertListPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('已撤销').first);
    await tester.pumpAndSettle();
    expect(find.text('课程 R1'), findsOneWidget);
    expect(find.text('课程 V1'), findsNothing);
  });

  testWidgets('shows error and retries', (tester) async {
    final api = _CertsApi(<Cert>[_cert('V1', 'valid')])..fail = true;
    await tester.pumpWidget(MaterialApp(home: CertListPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('证书加载失败'), findsOneWidget);
    api.fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('课程 V1'), findsOneWidget);
  });
}

