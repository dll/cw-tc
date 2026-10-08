import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';

/// 登录 / 注册（PRD v2 §9）：手机号 + 密码；注册需姓名 + 身份证号实名。
///
/// 铁律 3：身份证号只走 POST body（本页表单提交到后端 body，绝不进 URL）；
/// 登录账号 = 手机号；身份证号 = 实名 + 证书绑定，不用于登录。
class LoginPage extends StatefulWidget {
  const LoginPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  final _formKey = GlobalKey<FormState>();
  final _phone = TextEditingController();
  final _password = TextEditingController();
  final _name = TextEditingController();
  final _idCard = TextEditingController();

  bool _registerMode = false;
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _phone.dispose();
    _password.dispose();
    _name.dispose();
    _idCard.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      if (_registerMode) {
        await _api.register(
          name: _name.text.trim(),
          phone: _phone.text.trim(),
          idCard: _idCard.text.trim(),
          password: _password.text,
        );
      } else {
        await _api.login(phone: _phone.text.trim(), password: _password.text);
      }
      if (!mounted) return;
      context.pop();
    } catch (_) {
      if (!mounted) return;
      setState(() => _error = _registerMode ? '注册失败，请检查输入' : '手机号或密码错误');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(_registerMode ? '注册' : '登录')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              if (_registerMode) ...<Widget>[
                TextFormField(
                  controller: _name,
                  decoration: const InputDecoration(labelText: '姓名'),
                  validator: (v) => (v == null || v.trim().isEmpty) ? '请填写姓名' : null,
                ),
                const SizedBox(height: 12),
              ],
              TextFormField(
                controller: _phone,
                keyboardType: TextInputType.phone,
                decoration: const InputDecoration(labelText: '手机号'),
                validator: (v) => RegExp(r'^1[3-9]\d{9}$').hasMatch(v?.trim() ?? '') ? null : '手机号格式非法',
              ),
              const SizedBox(height: 12),
              if (_registerMode) ...<Widget>[
                TextFormField(
                  controller: _idCard,
                  decoration: const InputDecoration(labelText: '身份证号（实名）'),
                  validator: (v) => RegExp(r'^\d{17}[\dXx]$').hasMatch(v?.trim() ?? '') ? null : '身份证号格式非法',
                ),
                const SizedBox(height: 12),
              ],
              TextFormField(
                controller: _password,
                obscureText: true,
                decoration: const InputDecoration(labelText: '密码'),
                validator: (v) => (v == null || v.length < 6) ? '密码至少 6 位' : null,
              ),
              if (_error != null) ...<Widget>[
                const SizedBox(height: 12),
                Text(_error!, style: TextStyle(color: theme.colorScheme.error)),
              ],
              const SizedBox(height: 20),
              FilledButton(
                onPressed: _submitting ? null : _submit,
                child: Text(_submitting ? '提交中…' : (_registerMode ? '注册并登录' : '登录')),
              ),
              TextButton(
                onPressed: () => setState(() {
                  _registerMode = !_registerMode;
                  _error = null;
                }),
                child: Text(_registerMode ? '已有账号？去登录' : '没有账号？去注册'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
