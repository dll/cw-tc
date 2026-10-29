import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';

/// 忘记密码 / 管理员重置码：用一次性重置码自行设置新密码（不依赖短信）。
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
  final _password = TextEditingController();
  final _confirm = TextEditingController();
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _phone.dispose();
    _code.dispose();
    _password.dispose();
    _confirm.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await _api.resetPasswordWithCode(
        phone: _phone.text.trim(),
        code: _code.text.trim(),
        newPassword: _password.text,
      );
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
              Text('重置码由管理员发放（有效期 30 分钟）。管理员不会获知你的新密码。',
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
                    labelText: '重置码', hintText: '8 位数字，由管理员提供'),
                validator: (v) =>
                    RegExp(r'^[0-9]{6,12}$').hasMatch(v?.trim() ?? '')
                        ? null
                        : '请输入有效重置码',
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _password,
                obscureText: true,
                decoration: const InputDecoration(
                    labelText: '新密码', hintText: '8~20 位，同时包含字母和数字'),
                validator: (v) => _validPassword(v ?? '') ? null : '密码需 8~20 位且包含字母和数字',
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
                onPressed: _submitting ? null : _submit,
                child: Text(_submitting ? '提交中…' : '重置密码'),
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
