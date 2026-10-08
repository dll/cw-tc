import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/verify_result.dart';

/// 证书验真（PRD v2 §7.8 D-4）：公开页，无需登录，输入证书编号查真伪。
class VerifyPage extends StatefulWidget {
  const VerifyPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<VerifyPage> createState() => _VerifyPageState();
}

class _VerifyPageState extends State<VerifyPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  final _certNo = TextEditingController();
  bool _loading = false;
  VerifyResult? _result;
  String? _error;

  @override
  void dispose() {
    _certNo.dispose();
    super.dispose();
  }

  Future<void> _verify() async {
    final certNo = _certNo.text.trim();
    if (certNo.isEmpty) return;
    setState(() {
      _loading = true;
      _error = null;
      _result = null;
    });
    try {
      final result = await _api.verifyCert(certNo);
      if (mounted) setState(() => _result = result);
    } catch (_) {
      if (mounted) setState(() => _error = '证书不存在或格式错误');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('证书验真')),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: <Widget>[
          TextField(
            controller: _certNo,
            decoration: const InputDecoration(
              labelText: '证书编号',
              hintText: '如 CW-CN-A202600001',
              border: OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 12),
          FilledButton(onPressed: _loading ? null : _verify, child: Text(_loading ? '查询中…' : '验真')),
          if (_error != null) ...<Widget>[
            const SizedBox(height: 16),
            Text(_error!, style: TextStyle(color: theme.colorScheme.error)),
          ],
          if (_result != null) ...<Widget>[
            const SizedBox(height: 24),
            _VerifyResultCard(result: _result!),
          ],
        ],
      ),
    );
  }
}

class _VerifyResultCard extends StatelessWidget {
  const _VerifyResultCard({required this.result});

  final VerifyResult result;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final (IconData icon, String label, Color color) = switch (result.state) {
      'valid' => (Icons.verified, '有效', theme.colorScheme.primary),
      'expired' => (Icons.timelapse, '已过期', theme.disabledColor),
      'revoked' => (Icons.block, '已撤销', theme.colorScheme.error),
      _ => (Icons.help_outline, '未知', theme.disabledColor),
    };
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Icon(icon, color: color),
                const SizedBox(width: 8),
                Text(label, style: theme.textTheme.titleLarge?.copyWith(color: color)),
              ],
            ),
            const Divider(height: 24),
            _row(theme, '证书编号', result.certNo),
            _row(theme, '课程', result.course),
            _row(theme, '等级', result.level),
            _row(theme, '姓名', result.name),
            _row(theme, '签发时间', result.issuedAt),
            _row(theme, '有效期至', result.expiresAt),
            if (result.revokedReason != null && result.revokedReason!.isNotEmpty)
              _row(theme, '撤销原因', result.revokedReason!),
          ],
        ),
      ),
    );
  }

  Widget _row(ThemeData theme, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          SizedBox(width: 80, child: Text(label, style: TextStyle(color: theme.hintColor))),
          Expanded(child: Text(value)),
        ],
      ),
    );
  }
}
