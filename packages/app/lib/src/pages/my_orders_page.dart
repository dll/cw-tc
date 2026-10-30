import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../api/api_client.dart';
import '../models/order.dart';

/// 我的订单（PRD §7.13 L-2）：状态、继续支付、取消、退款进度。
class MyOrdersPage extends StatefulWidget {
  const MyOrdersPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<MyOrdersPage> createState() => _MyOrdersPageState();
}

class _MyOrdersPageState extends State<MyOrdersPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<MyOrder> _items = <MyOrder>[];
  bool _loading = true;
  bool _login = false;
  bool _failed = false;
  String _filter = 'all';
  String? _busyOrderNo;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _failed = false;
      _login = false;
    });
    try {
      final items = await _api.myOrders();
      if (!mounted) return;
      setState(() {
        _items = items;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _login = e is DioException && e.response?.statusCode == 401;
        _failed = true;
      });
    }
  }

  Future<void> _continuePay(MyOrder order) async {
    setState(() => _busyOrderNo = order.orderNo);
    try {
      final intent = await _api.continueOrderPayment(order.orderNo);
      if (!mounted) return;
      final paid = intent.channel == 'mock'
          ? await _showMockPay(intent)
          : await _showQrPay(intent);
      if (paid == true && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('支付成功，已开通学习权益')),
        );
        await _load();
      }
    } catch (e) {
      if (mounted) _snack(_errorMessage(e));
    } finally {
      if (mounted) setState(() => _busyOrderNo = null);
    }
  }

  Future<bool?> _showMockPay(OrderPaymentIntent intent) {
    return showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('模拟支付'),
        content: Text(
            '支付渠道未配置（开发 / staging）。订单 ${intent.orderNo} 应付 ${_money(intent.amountCents)}，可模拟支付成功。'),
        actions: <Widget>[
          TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: const Text('稍后支付')),
          FilledButton(
            onPressed: () async {
              try {
                await _api.mockPayOrder(intent.orderNo);
                if (dialogContext.mounted) {
                  Navigator.of(dialogContext).pop(true);
                }
              } catch (e) {
                if (dialogContext.mounted) {
                  ScaffoldMessenger.of(dialogContext)
                      .showSnackBar(SnackBar(content: Text(_errorMessage(e))));
                }
              }
            },
            child: const Text('模拟支付'),
          ),
        ],
      ),
    );
  }

  Future<bool?> _showQrPay(OrderPaymentIntent intent) {
    return showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('支付宝扫码支付'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            if (intent.qrCode != null)
              QrImageView(data: intent.qrCode!, size: 240),
            const SizedBox(height: 8),
            Text('应付 ${_money(intent.amountCents)}。付款后点击“我已支付”查询订单。'),
          ],
        ),
        actions: <Widget>[
          TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: const Text('稍后支付')),
          FilledButton(
            onPressed: () async {
              try {
                final latest = await _api.myOrder(intent.orderNo);
                if (dialogContext.mounted) {
                  Navigator.of(dialogContext).pop(latest.paid);
                }
              } catch (e) {
                if (dialogContext.mounted) {
                  ScaffoldMessenger.of(dialogContext)
                      .showSnackBar(SnackBar(content: Text(_errorMessage(e))));
                }
              }
            },
            child: const Text('我已支付'),
          ),
        ],
      ),
    );
  }

  Future<void> _cancel(MyOrder order) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('取消订单'),
        content: const Text('取消后订单关闭，可重新下单。确定取消？'),
        actions: <Widget>[
          TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: const Text('再想想')),
          FilledButton(
              onPressed: () => Navigator.of(dialogContext).pop(true),
              child: const Text('确定取消')),
        ],
      ),
    );
    if (ok != true) return;
    setState(() => _busyOrderNo = order.orderNo);
    try {
      await _api.cancelMyOrder(order.orderNo);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('订单已取消')),
        );
        await _load();
      }
    } catch (e) {
      if (mounted) _snack(_errorMessage(e));
    } finally {
      if (mounted) setState(() => _busyOrderNo = null);
    }
  }

  Future<void> _refund(MyOrder order) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('申请退款'),
        content: const Text(
            '退款将撤销本课程学习权益；若已发证将连带撤销证书。确定申请退款？'),
        actions: <Widget>[
          TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: const Text('再想想')),
          FilledButton(
              onPressed: () => Navigator.of(dialogContext).pop(true),
              child: const Text('确定退款')),
        ],
      ),
    );
    if (ok != true) return;
    setState(() => _busyOrderNo = order.orderNo);
    try {
      final refund = await _api.requestRefund(order.orderNo);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('退款申请已提交：${refund.statusLabel}')),
        );
        await _load();
      }
    } catch (e) {
      if (mounted) _snack(_errorMessage(e));
    } finally {
      if (mounted) setState(() => _busyOrderNo = null);
    }
  }

  void _snack(String message) {
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(message)));
  }

  String _errorMessage(Object error) {
    if (error is DioException) {
      final data = error.response?.data;
      if (data is Map<String, dynamic> && data['msg'] is String) {
        return data['msg'] as String;
      }
    }
    final text = error.toString();
    return text.startsWith('Exception: ')
        ? text.substring('Exception: '.length)
        : '操作失败，请稍后重试';
  }

  @override
  Widget build(BuildContext context) {
    final filtered = _filter == 'all'
        ? _items
        : _items.where((o) => o.status == _filter).toList();
    return Scaffold(
      appBar: AppBar(title: const Text('我的订单')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _failed
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      Text(_login ? '请先登录后查看订单' : '订单加载失败，请重试'),
                      TextButton(
                        onPressed: () async {
                          if (_login) {
                            await context.push('/login');
                            if (!mounted) return;
                          }
                          await _load();
                        },
                        child: Text(_login ? '去登录' : '重试'),
                      ),
                    ],
                  ),
                )
              : Column(
                  children: <Widget>[
                    SingleChildScrollView(
                      scrollDirection: Axis.horizontal,
                      padding: const EdgeInsets.all(12),
                      child: Row(
                        children: <Widget>[
                          for (final entry in <String, String>{
                            'all': '全部',
                            'pending': '待支付',
                            'paid': '已支付',
                            'closed': '已关闭',
                            'refunded': '已退款',
                          }.entries)
                            Padding(
                              padding: const EdgeInsets.only(right: 8),
                              child: ChoiceChip(
                                label: Text(entry.value),
                                selected: _filter == entry.key,
                                onSelected: (_) =>
                                    setState(() => _filter = entry.key),
                              ),
                            ),
                        ],
                      ),
                    ),
                    Expanded(
                      child: RefreshIndicator(
                        onRefresh: _load,
                        child: ListView(
                          physics: const AlwaysScrollableScrollPhysics(),
                          padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
                          children: filtered.isEmpty
                              ? <Widget>[
                                  const Padding(
                                    padding: EdgeInsets.all(40),
                                    child: Center(child: Text('暂无订单')),
                                  ),
                                ]
                              : <Widget>[
                                  for (final order in filtered)
                                    _orderCard(context, order),
                                ],
                        ),
                      ),
                    ),
                  ],
                ),
    );
  }

  Widget _orderCard(BuildContext context, MyOrder order) {
    final theme = Theme.of(context);
    final busy = _busyOrderNo == order.orderNo;
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Expanded(
                  child: Text(order.courseName,
                      style: theme.textTheme.titleMedium),
                ),
                Chip(
                  label: Text(order.statusLabel),
                  visualDensity: VisualDensity.compact,
                ),
              ],
            ),
            const SizedBox(height: 6),
            Text('订单号：${order.orderNo}', style: theme.textTheme.bodySmall),
            const SizedBox(height: 2),
            Text(
              '${_money(order.amountCents)} · ${order.channelLabel} · ${_time(order.createdAt)}',
              style: theme.textTheme.bodySmall,
            ),
            if (order.refund != null) ...<Widget>[
              const SizedBox(height: 8),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: order.refund!.status == 'failed'
                      ? Colors.red.shade50
                      : theme.colorScheme.secondaryContainer.withOpacity(0.35),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      '${order.refund!.statusLabel} · ${_money(order.refund!.amountCents)}',
                      style: theme.textTheme.bodyMedium,
                    ),
                    if (order.refund!.finishedAt != null)
                      Text('完成时间：${_time(order.refund!.finishedAt)}',
                          style: theme.textTheme.bodySmall),
                    if (order.refund!.failReason != null)
                      Text('失败原因：${order.refund!.failReason}',
                          style: theme.textTheme.bodySmall
                              ?.copyWith(color: Colors.red.shade700)),
                  ],
                ),
              ),
            ],
            if (order.certRevoked) ...<Widget>[
              const SizedBox(height: 6),
              Text('该课程证书已因退款撤销',
                  style: theme.textTheme.bodySmall
                      ?.copyWith(color: Colors.orange.shade800)),
            ],
            if (order.paid && !order.canRefund) ...<Widget>[
              const SizedBox(height: 6),
              Text(
                order.certIssued
                    ? '已发证，退款请联系客服'
                    : '已超过 7 日自助退款窗口，请联系客服',
                style: theme.textTheme.bodySmall
                    ?.copyWith(color: theme.colorScheme.outline),
              ),
            ],
            const SizedBox(height: 10),
            Wrap(
              spacing: 8,
              runSpacing: 4,
              children: <Widget>[
                if (order.canContinuePay)
                  FilledButton(
                    onPressed: busy ? null : () => _continuePay(order),
                    child: Text(busy ? '处理中…' : '继续支付'),
                  ),
                if (order.canCancel)
                  OutlinedButton(
                    onPressed: busy ? null : () => _cancel(order),
                    child: const Text('取消订单'),
                  ),
                if (order.canEnterCourse)
                  FilledButton(
                    onPressed: () => context.push(
                        '/learn/${Uri.encodeComponent(order.courseCode)}/1'),
                    child: const Text('进入课程'),
                  ),
                if (order.canRefund)
                  OutlinedButton(
                    onPressed: busy ? null : () => _refund(order),
                    child: const Text('申请退款'),
                  ),
                TextButton(
                  onPressed: () => _showDetail(order),
                  child: const Text('查看详情'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showDetail(MyOrder order) {
    showModalBottomSheet<void>(
      context: context,
      builder: (sheetContext) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
          children: <Widget>[
            Text('订单详情', style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 12),
            _detailRow('订单号', order.orderNo),
            _detailRow('课程', order.courseName),
            _detailRow('课程码', order.courseCode),
            _detailRow('金额', _money(order.amountCents)),
            _detailRow('状态', order.statusLabel),
            _detailRow('支付渠道', order.channelLabel),
            _detailRow('下单时间', _time(order.createdAt)),
            _detailRow('支付时间', _time(order.paidAt)),
            _detailRow('关闭时间', _time(order.expiredAt)),
            _detailRow('退款时间', _time(order.refundedAt)),
            if (order.refundableUntil != null)
              _detailRow('退款窗口至', _time(order.refundableUntil)),
            if (order.refund != null) ...<Widget>[
              const Divider(height: 24),
              _detailRow('退款单号', order.refund!.refundNo),
              _detailRow('退款状态', order.refund!.statusLabel),
              _detailRow('退款金额', _money(order.refund!.amountCents)),
              _detailRow('退款时间', _time(order.refund!.finishedAt)),
              if (order.refund!.failReason != null)
                _detailRow('失败原因', order.refund!.failReason!),
            ],
          ],
        ),
      ),
    );
  }

  Widget _detailRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          SizedBox(width: 88, child: Text(label)),
          Expanded(child: SelectableText(value.isEmpty ? '—' : value)),
        ],
      ),
    );
  }

  String _money(int cents) => '¥${(cents / 100).toStringAsFixed(2)}';

  String _time(String? value) {
    if (value == null || value.isEmpty) return '—';
    return value.replaceFirst('T', ' ').split('.').first;
  }
}

