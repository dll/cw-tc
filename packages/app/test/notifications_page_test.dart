import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/message.dart';
import 'package:cw_tc_app/src/pages/notifications_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _MessagesApi extends ApiClient {
  _MessagesApi(this.items);

  List<AppMessage> items;
  bool optOut = false;
  int markReadCalls = 0;
  int markAllCalls = 0;

  int get unread => items.where((m) => !m.read).length;

  @override
  Future<MessagePage> messages() async => MessagePage(
        items: items,
        unread: unread,
        prefs: MessagePrefs(broadcastOptOut: optOut),
      );

  @override
  Future<void> markNotificationRead(int id) async {
    markReadCalls++;
    items = items
        .map((m) => m.id == id
            ? AppMessage(
                id: m.id,
                type: m.type,
                title: m.title,
                body: m.body,
                link: m.link,
                read: true,
                createdAt: m.createdAt)
            : m)
        .toList();
  }

  @override
  Future<void> markAllMessagesRead() async {
    markAllCalls++;
    items = items
        .map((m) => AppMessage(
            id: m.id,
            type: m.type,
            title: m.title,
            body: m.body,
            link: m.link,
            read: true,
            createdAt: m.createdAt))
        .toList();
  }

  @override
  Future<void> setBroadcastOptOut(bool value) async => optOut = value;
}

AppMessage _msg(int id, String type, String title, {bool read = false}) =>
    AppMessage(
      id: id,
      type: type,
      title: title,
      body: '$title 正文',
      link: '/me/orders',
      read: read,
      createdAt: '2026-10-01T12:00:00Z',
    );

void main() {
  testWidgets('shows unread count and marks all read', (tester) async {
    final api = _MessagesApi(<AppMessage>[
      _msg(1, 'system', '账号安全'),
      _msg(2, 'order', '支付成功'),
    ]);
    await tester.pumpWidget(MaterialApp(home: NotificationsPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('消息（2）'), findsOneWidget);
    await tester.tap(find.text('全部已读'));
    await tester.pumpAndSettle();
    expect(api.markAllCalls, 1);
    expect(find.text('消息'), findsOneWidget);
  });

  testWidgets('filters categories and unread', (tester) async {
    final api = _MessagesApi(<AppMessage>[
      _msg(1, 'system', '系统消息', read: true),
      _msg(2, 'broadcast', '广播消息'),
    ]);
    await tester.pumpWidget(MaterialApp(home: NotificationsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('广播').first);
    await tester.pumpAndSettle();
    expect(find.text('广播消息'), findsOneWidget);
    expect(find.text('系统消息'), findsNothing);
    await tester.tap(find.text('全部').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('只看未读'));
    await tester.pumpAndSettle();
    expect(find.text('广播消息'), findsOneWidget);
    expect(find.text('系统消息'), findsNothing);
  });

  testWidgets('broadcast preference toggles opt out', (tester) async {
    final api = _MessagesApi(<AppMessage>[_msg(1, 'system', '系统消息')]);
    await tester.pumpWidget(MaterialApp(home: NotificationsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    expect(api.optOut, isTrue);
  });

  testWidgets('opening message marks it read', (tester) async {
    final api = _MessagesApi(<AppMessage>[_msg(1, 'order', '支付成功')]);
    await tester.pumpWidget(MaterialApp(home: NotificationsPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('支付成功'));
    await tester.pumpAndSettle();
    expect(api.markReadCalls, 1);
    expect(find.text('支付成功'), findsWidgets);
    await tester.tap(find.text('关闭'));
    await tester.pumpAndSettle();
  });
}
