import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';

/// 忘记密码：管理员授权后，学员可用站内重置码或身份证验证重置密码。
class ResetPasswordPage extends StatefulWidget {
  const ResetPasswordPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<ResetPasswordPage> createState() => _ResetPasswordPageState();
}

class _ResetPasswordPageState extends State<ResetPasswordPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  final _formKey = GlobalKey<FormState>();
  final _phone = TextEditingController();
  final _code = TextEditingController();
  final _idCard = TextEditingController();
  final _password = TextEditingController();
  final _confirm = TextEditingController();
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _phone.dispose();
    _code.dispose();
    _idCard.dispose();
    _password.dispose();
    _confirm.dispose();
    super.dispose();
  }

  Future<void> _reset({required bool byIdentity}) async {
    if (!_formKey.currentState!.validate()) return;
    if (byIdentity && !_validIDCard(_idCard.text.trim())) {
      setState(() => _error = '身份证号格式或校验位不正确');
      return;
    }
    if (!byIdentity && _code.text.trim().isEmpty) {
      setState(() => _error = '请输入站内重置码');
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      if (byIdentity) {
        await _api.resetPasswordByIdentity(
          phone: _phone.text.trim(),
          idCard: _idCard.text.trim(),
          newPassword: _password.text,
        );
      } else {
        await _api.resetPasswordWithCode(
          phone: _phone.text.trim(),
          code: _code.text.trim(),
          newPassword: _password.text,
        );
      }
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('密码已重置，请用新密码登录')));
      context.pop();
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = e.toString().replaceFirst('Exception: ', ''));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('重置密码')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              Text(
                  '管理员授权后，重置码会通过站内消息发送给你；也可直接用身份证验证重置，无需管理员转交。',
                  style: theme.textTheme.bodySmall),
              const SizedBox(height: 16),
              TextFormField(
                controller: _phone,
                keyboardType: TextInputType.phone,
                decoration: const InputDecoration(labelText: '手机号'),
                validator: (v) =>
                    RegExp(r'^1[3-9][0-9]{9}$').hasMatch(v?.trim() ?? '')
                        ? null
                        : '手机号格式非法',
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _code,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                    labelText: '站内重置码（可在「我的 → 消息」查看）',
                    hintText: '8 位数字'),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _idCard,
                decoration: const InputDecoration(
                    labelText: '身份证号（用身份证验证时填写）',
                    hintText: '18 位，仅用于身份核验'),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _password,
                obscureText: true,
                decoration: const InputDecoration(
                    labelText: '新密码', hintText: '8~20 位，同时包含字母和数字'),
                validator: (v) =>
                    _validPassword(v ?? '') ? null : '密码需 8~20 位且包含字母和数字',
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _confirm,
                obscureText: true,
                decoration: const InputDecoration(labelText: '确认新密码'),
                validator: (v) => v == _password.text ? null : '两次输入不一致',
              ),
              if (_error != null) ...<Widget>[
                const SizedBox(height: 12),
                Text(_error!, style: TextStyle(color: theme.colorScheme.error)),
              ],
              const SizedBox(height: 20),
              FilledButton(
                onPressed: _submitting ? null : () => _reset(byIdentity: false),
                child: Text(_submitting ? '提交中…' : '用重置码重置'),
              ),
              const SizedBox(height: 8),
              OutlinedButton(
                onPressed: _submitting ? null : () => _reset(byIdentity: true),
                child: const Text('用身份证验证重置'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

bool _validPassword(String value) =>
    value.length >= 8 &&
    value.length <= 20 &&
    RegExp(r'[A-Za-z]').hasMatch(value) &&
    RegExp(r'[0-9]').hasMatch(value);

bool _validIDCard(String value) {
  if (!RegExp(r'^\d{17}[\dXx]$').hasMatch(value)) return false;
  const weights = <int>[7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2];
  const checks = '10X98765432';
  var sum = 0;
  for (var i = 0; i < 17; i++) {
    sum += int.parse(value[i]) * weights[i];
  }
  return value[17].toUpperCase() == checks[sum % 11];
}
