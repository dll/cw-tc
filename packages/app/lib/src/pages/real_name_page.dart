import 'package:flutter/material.dart';

import '../api/api_client.dart';

/// 补充实名（PRD v2 §9.5）：仅未实名时设置身份证号；服务端加密存储，页面只显示脱敏结果。
class RealNamePage extends StatefulWidget {
  const RealNamePage({super.key, this.api});

  final ApiClient? api;

  @override
  State<RealNamePage> createState() => _RealNamePageState();
}

class _RealNamePageState extends State<RealNamePage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  final GlobalKey<FormState> _formKey = GlobalKey<FormState>();
  final TextEditingController _idCard = TextEditingController();
  bool _loading = true;
  bool _busy = false;
  bool _verified = false;
  String _masked = '';
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _idCard.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final me = await _api.me();
      if (!mounted) return;
      setState(() {
        _verified = me['identity_verified'] == true;
        _masked = me['identity_masked'] as String? ?? '';
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await _api.completeIdentity(idCard: _idCard.text.trim());
      if (!mounted) return;
      setState(() => _verified = true);
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('实名信息已提交')));
      await _load();
    } catch (e) {
      final msg = e.toString().replaceFirst('Exception: ', '');
      if (mounted) {
        setState(() => _error = msg.isEmpty ? '提交失败，请稍后重试' : msg);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  String? _validateIDCard(String? value) {
    final text = (value ?? '').trim();
    if (text.length != 18) return '身份证号需为 18 位';
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('实名信息')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: const EdgeInsets.all(20),
              children: <Widget>[
                if (_verified) ...<Widget>[
                  Icon(Icons.verified_user,
                      size: 56, color: theme.colorScheme.primary),
                  const SizedBox(height: 12),
                  Text('已完成实名', style: theme.textTheme.titleLarge),
                  const SizedBox(height: 8),
                  Text('证件号：$_masked'),
                  const SizedBox(height: 12),
                  Text('身份证号仅用于证书绑定，平台展示一律脱敏。',
                      style: theme.textTheme.bodySmall),
                ] else ...<Widget>[
                  Text('领取证书前需补充实名信息。',
                      style: theme.textTheme.titleMedium),
                  const SizedBox(height: 8),
                  Text('平台按你申报的姓名与身份证号留档备查；证书姓名取自注册姓名。',
                      style: theme.textTheme.bodySmall),
                  const SizedBox(height: 20),
                  Form(
                    key: _formKey,
                    child: TextFormField(
                      controller: _idCard,
                      textCapitalization: TextCapitalization.characters,
                      decoration: const InputDecoration(
                          labelText: '身份证号',
                          border: OutlineInputBorder()),
                      validator: _validateIDCard,
                    ),
                  ),
                  if (_error != null) ...<Widget>[
                    const SizedBox(height: 12),
                    Text(_error!,
                        style: TextStyle(color: theme.colorScheme.error)),
                  ],
                  const SizedBox(height: 20),
                  FilledButton(
                    onPressed: _busy ? null : _submit,
                    child: Text(_busy ? '提交中…' : '提交实名'),
                  ),
                ],
              ],
            ),
    );
  }
}
