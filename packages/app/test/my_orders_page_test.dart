import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/order.dart';
import 'package:cw_tc_app/src/pages/my_orders_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _OrdersApi extends ApiClient {
  _OrdersApi(this.items);

  List<MyOrder> items;
  int calls = 0;
  bool fail = false;
  bool cancelled = false;
  bool refunded = false;
  bool mockPaid = false;

  @override
  Future<List<MyOrder>> myOrders() async {
    calls++;
    if (fail) throw Exception('offline');
    return items;
  }

  @override
  Future<OrderPaymentIntent> continueOrderPayment(String orderNo) async =>
      OrderPaymentIntent(
          orderNo: orderNo, channel: 'mock', amountCents: 12900);

  @override
  Future<void> mockPayOrder(String orderNo) async => mockPaid = true;

  @override
  Future<MyOrder> myOrder(String orderNo) async => items.first;

  @override
  Future<void> cancelMyOrder(String orderNo) async => cancelled = true;

  @override
  Future<OrderRefund> requestRefund(String orderNo, {String? reason}) async {
    refunded = true;
    return const OrderRefund(
        refundNo: 'R1',
        amountCents: 12900,
        status: 'success',
        reason: '学员自助退款',
        createdAt: '2026-10-02T00:00:00Z');
  }
}

MyOrder _order(
  String no,
  String name, {
  String status = 'pending',
  bool canContinue = true,
  bool canCancel = true,
  bool canRefund = false,
  bool canEnter = false,
  bool examTaken = false,
  bool certIssued = false,
  bool certRevoked = false,
  OrderRefund? refund,
}) =>
    MyOrder(
      orderNo: no,
      courseCode: 'C-$no',
      courseName: name,
      amountCents: 12900,
      status: status,
      channel: 'alipay',
      createdAt: '2026-10-01T12:00:00Z',
      expiredAt: '2026-10-01T12:30:00Z',
      canContinuePay: canContinue,
      canCancel: canCancel,
      canRefund: canRefund,
      canEnterCourse: canEnter,
      examTaken: examTaken,
      certIssued: certIssued,
      certRevoked: certRevoked,
      refundableUntil: canRefund ? '2026-10-08T12:00:00Z' : null,
      refund: refund,
    );

void main() {
  testWidgets('pending order can cancel and continue pay', (tester) async {
    final api = _OrdersApi(<MyOrder>[_order('1', '待支付课程')]);
    await tester.pumpWidget(MaterialApp(home: MyOrdersPage(api: api)));
    await tester.pumpAndSettle();

    expect(find.text('继续支付'), findsOneWidget);
    expect(find.text('取消订单'), findsOneWidget);
    await tester.tap(find.text('取消订单'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('确定取消'));
    await tester.pumpAndSettle();
    expect(api.cancelled, isTrue);
  });

  testWidgets('mock payment flow marks paid', (tester) async {
    final api = _OrdersApi(<MyOrder>[_order('1', '待支付课程')]);
    await tester.pumpWidget(MaterialApp(home: MyOrdersPage(api: api)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('继续支付'));
    await tester.pumpAndSettle();
    expect(find.text('模拟支付'), findsWidgets);
    await tester.tap(find.text('模拟支付').last);
    await tester.pumpAndSettle();
    expect(api.mockPaid, isTrue);
  });

  testWidgets('paid order shows refund only when allowed', (tester) async {
    final api = _OrdersApi(<MyOrder>[
      _order('1', '可退款课程',
          status: 'paid', canContinue: false, canCancel: false,
          canRefund: true, canEnter: true),
    ]);
    await tester.pumpWidget(MaterialApp(home: MyOrdersPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('进入课程'), findsOneWidget);
    expect(find.text('申请退款'), findsOneWidget);
    await tester.tap(find.text('申请退款'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('确定退款'));
    await tester.pumpAndSettle();
    expect(api.refunded, isTrue);
  });

  testWidgets('issued certificate hides self-service refund', (tester) async {
    final api = _OrdersApi(<MyOrder>[
      _order('1', '已发证课程',
          status: 'paid', canContinue: false, canCancel: false,
          canRefund: false, canEnter: true, certIssued: true),
    ]);
    await tester.pumpWidget(MaterialApp(home: MyOrdersPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('申请退款'), findsNothing);
    expect(find.text('已发证，退款请联系客服'), findsOneWidget);
  });

  testWidgets('refunded order shows progress and revoked certificate',
      (tester) async {
    final api = _OrdersApi(<MyOrder>[
      _order('1', '已退款课程',
          status: 'refunded',
          canContinue: false,
          canCancel: false,
          certRevoked: true,
          refund: const OrderRefund(
              refundNo: 'R1',
              amountCents: 12900,
              status: 'success',
              reason: '学员自助退款',
              createdAt: '2026-10-02T00:00:00Z',
              finishedAt: '2026-10-02T00:01:00Z')),
    ]);
    await tester.pumpWidget(MaterialApp(home: MyOrdersPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('已退款 · ¥129.00'), findsOneWidget);
    expect(find.text('该课程证书已因退款撤销'), findsOneWidget);
  });

  testWidgets('status filter hides other orders', (tester) async {
    final api = _OrdersApi(<MyOrder>[
      _order('1', '待支付课程'),
      _order('2', '已支付课程',
          status: 'paid', canContinue: false, canCancel: false, canEnter: true),
    ]);
    await tester.pumpWidget(MaterialApp(home: MyOrdersPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('待支付课程'), findsOneWidget);
    expect(find.text('已支付课程'), findsOneWidget);
    await tester.tap(find.text('待支付').first);
    await tester.pumpAndSettle();
    expect(find.text('待支付课程'), findsOneWidget);
    expect(find.text('已支付课程'), findsNothing);
  });

  testWidgets('shows error and retries loading', (tester) async {
    final api = _OrdersApi(<MyOrder>[_order('1', '课程')])..fail = true;
    await tester.pumpWidget(MaterialApp(home: MyOrdersPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('订单加载失败，请重试'), findsOneWidget);
    api.fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('课程'), findsOneWidget);
    expect(api.calls, 2);
  });
}
