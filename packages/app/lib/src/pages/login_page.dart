import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:dio/dio.dart';

import '../api/api_client.dart';

/// 登录 / 注册（PRD v2 §9）：手机号 + 密码；注册需姓名，身份证号可选。
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
        final token = await _api.register(
          name: _name.text.trim(),
          phone: _phone.text.trim(),
          idCard: _idCard.text.trim(),
          password: _password.text,
        );
        if (token.isEmpty) {
          if (!mounted) return;
          setState(() => _error = '注册已提交，运营管理员审核手机号后即可登录。');
          return;
        }
      } else {
        await _api.login(phone: _phone.text.trim(), password: _password.text);
      }
      if (!mounted) return;
      context.pop();
    } catch (error) {
      if (!mounted) return;
      String message = _registerMode ? '注册失败，请检查输入' : '登录失败，请稍后重试';
      if (error is DioException) {
        final body = error.response?.data;
        if (body is Map && body['msg'] is String) {
          message = body['msg'] as String;
        }
      } else {
        final detail = error.toString();
        if (detail.startsWith('Exception: ')) message = detail.substring(11);
      }
      setState(() => _error = message);
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
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? '请填写姓名' : null,
                ),
                const SizedBox(height: 12),
              ],
              TextFormField(
                controller: _phone,
                keyboardType: TextInputType.phone,
                decoration: const InputDecoration(labelText: '手机号'),
                validator: (v) =>
                    _validPhone(v?.trim() ?? '') ? null : '手机号格式非法',
              ),
              const SizedBox(height: 12),
              if (_registerMode) ...<Widget>[
                TextFormField(
                  controller: _idCard,
                  decoration: const InputDecoration(labelText: '身份证号（选填，领取证书前需补充）'),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty || _validIDCard(v.trim())) ? null : '身份证号格式或校验位非法',
                ),
                const SizedBox(height: 12),
              ],
              TextFormField(
                controller: _password,
                obscureText: true,
                decoration: const InputDecoration(labelText: '密码'),
                validator: (v) => _registerMode
                    ? (_validPassword(v ?? '') ? null : '密码需为 8~20 位且同时包含字母和数字')
                    : ((v == null || v.isEmpty) ? '请输入密码' : null),
              ),
              if (_error != null) ...<Widget>[
                const SizedBox(height: 12),
                Text(_error!, style: TextStyle(color: theme.colorScheme.error)),
              ],
              const SizedBox(height: 20),
              FilledButton(
                onPressed: _submitting ? null : _submit,
                  child: Text(
                    _submitting ? '提交中…' : (_registerMode ? '提交注册审核' : '登录')),
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

bool _validPhone(String value) => RegExp(r'^1[3-9][0-9]{9}$').hasMatch(value);

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
  if (value[17].toUpperCase() != checks[sum % 11]) return false;
  final month = int.parse(value.substring(10, 12));
  final day = int.parse(value.substring(12, 14));
  final year = int.parse(value.substring(6, 10));
  if (year < 1900 || month < 1 || month > 12 || day < 1 || day > 31) {
    return false;
  }
  final date = DateTime.utc(year, month, day);
  if (date.year != year ||
      date.month != month ||
      date.day != day ||
      date.isAfter(DateTime.now().toUtc())) return false;
  const regions = <String>{
    '11',
    '12',
    '13',
    '14',
    '15',
    '21',
    '22',
    '23',
    '31',
    '32',
    '33',
    '34',
    '35',
    '36',
    '37',
    '41',
    '42',
    '43',
    '44',
    '45',
    '46',
    '50',
    '51',
    '52',
    '53',
    '54',
    '61',
    '62',
    '63',
    '64',
    '65',
    '71',
    '81',
    '82'
  };
  return regions.contains(value.substring(0, 2));
}
