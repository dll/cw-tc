import 'dart:convert';
import 'dart:typed_data';

import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

class _OrderAdapter implements HttpClientAdapter {
  final requests = <RequestOptions>[];

  @override
  void close({bool force = false}) {}

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    requests.add(options);
    Map<String, dynamic> data;
    if (options.path == '/api/v1/me/orders' && options.method == 'GET') {
      data = <String, dynamic>{
        'items': <Map<String, dynamic>>[
          <String, dynamic>{
            'order_no': 'O1',
            'course_code': 'C1',
            'course_name': '课程一',
            'amount_cents': 12900,
            'status': 'paid',
            'channel': 'alipay',
            'created_at': '2026-10-01T12:00:00Z',
            'paid_at': '2026-10-01T12:01:00Z',
            'expired_at': '2026-10-01T12:30:00Z',
            'can_continue_pay': false,
            'can_cancel': false,
            'can_refund': true,
            'can_enter_course': true,
            'exam_taken': false,
            'cert_issued': false,
            'cert_revoked': false,
            'refundable_until': '2026-10-08T12:01:00Z',
          },
        ],
      };
    } else if (options.path.endsWith('/pay')) {
      data = <String, dynamic>{
        'order_no': 'O1',
        'channel': 'mock',
        'amount_cents': 12900,
      };
    } else if (options.path.endsWith('/refund') && options.method == 'POST') {
      data = <String, dynamic>{
        'refund_no': 'R1',
        'amount_cents': 12900,
        'status': 'success',
      };
    } else {
      data = <String, dynamic>{'status': 'closed'};
    }
    return ResponseBody.fromString(
      jsonEncode(<String, dynamic>{'code': 0, 'data': data}),
      200,
      headers: <String, List<String>>{
        'content-type': <String>['application/json'],
      },
    );
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('my orders parses actions and uses me order routes', () async {
    FlutterSecureStorage.setMockInitialValues(
        <String, String>{'cw_tc_session': 'test-token'});
    final dio = Dio();
    final adapter = _OrderAdapter();
    dio.httpClientAdapter = adapter;
    final api = ApiClient(dio: dio);

    final items = await api.myOrders();
    final order = items.single;
    expect(order.orderNo, 'O1');
    expect(order.courseCode, 'C1');
    expect(order.canRefund, isTrue);
    expect(order.canEnterCourse, isTrue);
    expect(order.refundableUntil, '2026-10-08T12:01:00Z');

    final intent = await api.continueOrderPayment('O1');
    expect(intent.channel, 'mock');
    expect(intent.amountCents, 12900);
    await api.cancelMyOrder('O1');
    final refund = await api.requestRefund('O1');
    expect(refund.refundNo, 'R1');
    expect(refund.statusLabel, '已退款');

    expect(adapter.requests[0].path, '/api/v1/me/orders');
    expect(adapter.requests[1].path, '/api/v1/me/orders/O1/pay');
    expect(adapter.requests[1].method, 'POST');
    expect(adapter.requests[2].path, '/api/v1/me/orders/O1/cancel');
    expect(adapter.requests[3].path, '/api/v1/me/orders/O1/refund');
    expect(adapter.requests[3].method, 'POST');
    expect(adapter.requests[0].headers['Authorization'], 'Bearer test-token');
  });
}
