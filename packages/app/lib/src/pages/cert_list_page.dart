import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/cert.dart';
import 'cert_viewer_page.dart';

/// 我的证书（PRD §7.13 L-4）：有效 / 即将到期 / 已过期 / 已撤销。
class CertListPage extends StatefulWidget {
  const CertListPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<CertListPage> createState() => _CertListPageState();
}

class _CertListPageState extends State<CertListPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<Cert> _items = <Cert>[];
  bool _loading = true;
  bool _failed = false;
  bool _login = false;
  String _filter = 'all';

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
      final items = await _api.myCerts();
      if (!mounted) return;
      setState(() {
        _items = items;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _failed = true;
        _login = e is DioException && e.response?.statusCode == 401;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final filtered = _filter == 'all'
        ? _items
        : _items.where((c) => c.displayStatus == _filter).toList();
    return Scaffold(
      appBar: AppBar(title: const Text('我的证书')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _failed
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      Text(_login ? '请先登录后查看证书' : '证书加载失败'),
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
                            'valid': '有效',
                            'expiring': '即将到期',
                            'expired': '已过期',
                            'revoked': '已撤销',
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
                        child: ListView.separated(
                          physics: const AlwaysScrollableScrollPhysics(),
                          padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
                          itemCount: filtered.isEmpty ? 1 : filtered.length,
                          separatorBuilder: (_, __) =>
                              const SizedBox(height: 8),
                          itemBuilder: (BuildContext context, int i) {
                            if (filtered.isEmpty) {
                              return const Padding(
                                padding: EdgeInsets.all(40),
                                child: Center(
                                    child: Text('暂无证书，通过测评后领取')),
                              );
                            }
                            return _certCard(filtered[i]);
                          },
                        ),
                      ),
                    ),
                  ],
                ),
    );
  }

  Widget _certCard(Cert cert) {
    final theme = Theme.of(context);
    final color = cert.revoked
        ? theme.disabledColor
        : cert.displayStatus == 'expiring'
            ? Colors.orange.shade800
            : cert.displayStatus == 'expired'
                ? theme.colorScheme.outline
                : theme.colorScheme.primary;
    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Icon(cert.revoked ? Icons.block : Icons.verified,
                    color: color),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(cert.courseName,
                      style: theme.textTheme.titleMedium),
                ),
                Chip(
                  label: Text(cert.statusLabel),
                  visualDensity: VisualDensity.compact,
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text('${cert.certNo} · ${_levelLabel(cert.level)} · ${cert.score.toStringAsFixed(1)} 分',
                style: theme.textTheme.bodySmall),
            Text('签发 ${_date(cert.issuedAt)} · 有效期至 ${_date(cert.expiresAt)}',
                style: theme.textTheme.bodySmall),
            if (cert.revoked) ...<Widget>[
              const SizedBox(height: 6),
              Text('撤销原因：${cert.revokedReasonLabel}',
                  style: theme.textTheme.bodySmall
                      ?.copyWith(color: Colors.red.shade700)),
            ],
            const SizedBox(height: 8),
            Align(
              alignment: Alignment.centerRight,
              child: FilledButton.tonalIcon(
                onPressed: cert.canDownload
                    ? () => Navigator.of(context).push(
                          MaterialPageRoute<void>(
                            builder: (_) => CertViewerPage(
                                certNo: cert.certNo, api: _api),
                          ),
                        )
                    : null,
                icon: const Icon(Icons.picture_as_pdf_outlined),
                label: Text(cert.canDownload ? '查看 / 打印' : '不可下载'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _levelLabel(String level) {
    switch (level) {
      case 'L1':
        return '初级';
      case 'L2':
        return '中级';
      case 'L3':
        return '高级';
      default:
        return level;
    }
  }

  String _date(String value) {
    if (value.isEmpty) return '—';
    return value.replaceFirst('T', ' ').split('.').first;
  }
}
