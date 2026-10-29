import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';

/// 账号与安全（PRD v2 §7.13 L-6）：改密 / 换绑手机号 / 登录设备 / 注销账号。
class AccountSecurityPage extends StatefulWidget {
  const AccountSecurityPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<AccountSecurityPage> createState() => _AccountSecurityPageState();
}

class _AccountSecurityPageState extends State<AccountSecurityPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late final Future<Map<String, dynamic>> _me = _api.me();
  late Future<List<Map<String, dynamic>>> _devices = _api.devices();

  final TextEditingController _oldPassword = TextEditingController();
  final TextEditingController _newPassword = TextEditingController();
  final TextEditingController _confirmPassword = TextEditingController();
  final TextEditingController _oldCode = TextEditingController();
  final TextEditingController _newPhone = TextEditingController();
  final TextEditingController _newCode = TextEditingController();
  bool _busy = false;

  @override
  void dispose() {
    _oldPassword.dispose();
    _newPassword.dispose();
    _confirmPassword.dispose();
    _oldCode.dispose();
    _newPhone.dispose();
    _newCode.dispose();
    super.dispose();
  }

  void _toast(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  String _friendly(Object error) =>
      error.toString().replaceFirst('Exception: ', '');

  void _reloadDevices() => setState(() => _devices = _api.devices());

  Future<void> _changePassword() async {
    if (_newPassword.text.length < 8 || _newPassword.text.length > 20) {
      _toast('新密码需 8~20 位');
      return;
    }
    if (_newPassword.text != _confirmPassword.text) {
      _toast('两次输入的新密码不一致');
      return;
    }
    setState(() => _busy = true);
    try {
      await _api.changePassword(
          oldPassword: _oldPassword.text, newPassword: _newPassword.text);
      // 服务端已撤销全部会话：本地凭据一并清除，要求重新登录。
      await _api.logout();
      _toast('密码已修改，请重新登录');
      if (mounted) context.go('/login');
    } catch (e) {
      _toast(_friendly(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _rebindPhone() async {
    setState(() => _busy = true);
    try {
      await _api.rebindPhone(
        oldCode: _oldCode.text.trim(),
        phone: _newPhone.text.trim(),
        newCode: _newCode.text.trim(),
      );
      await _api.logout();
      _toast('手机号已换绑，请用新手机号登录');
      if (mounted) context.go('/login');
    } catch (e) {
      _toast(_friendly(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _revokeDevice(int id) async {
    try {
      await _api.revokeDevice(id);
      _reloadDevices();
      _toast('已退出该设备');
    } catch (e) {
      _toast(_friendly(e));
    }
  }

  Future<void> _logoutAll() async {
    try {
      await _api.logoutAll();
      _toast('已退出全部设备');
      if (mounted) context.go('/login');
    } catch (e) {
      _toast(_friendly(e));
    }
  }

  Future<void> _deleteAccount() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (BuildContext dialogContext) => AlertDialog(
        title: const Text('注销账号'),
        content: const Text(
            '注销后姓名与手机号将被匿名化，身份证信息删除；证书台账依法留存，仍可按编号验真。此操作不可恢复。'),
        actions: <Widget>[
          TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: const Text('取消')),
          FilledButton(
              onPressed: () => Navigator.of(dialogContext).pop(true),
              child: const Text('确认注销')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await _api.deleteAccount();
      _toast('账号已注销');
      if (mounted) context.go('/');
    } catch (e) {
      _toast(_friendly(e));
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('账号与安全')),
      body: FutureBuilder<Map<String, dynamic>>(
        future: _me,
        builder: (BuildContext context, AsyncSnapshot<Map<String, dynamic>> snap) {
          if (snap.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snap.hasError) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  const Text('请先登录'),
                  const SizedBox(height: 12),
                  FilledButton(
                      onPressed: () => context.push('/login'),
                      child: const Text('去登录')),
                ],
              ),
            );
          }
          final me = snap.data ?? const <String, dynamic>{};
          return ListView(
            padding: const EdgeInsets.all(16),
            children: <Widget>[
              Card(
                child: ListTile(
                  leading: const CircleAvatar(child: Icon(Icons.person)),
                  title: Text(me['name'] as String? ?? '学员'),
                  subtitle: Text(
                      '学号 ${me['student_no'] ?? '-'} · ${me['phone'] ?? '-'}'),
                ),
              ),
              _sectionTitle(theme, '修改密码'),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(children: <Widget>[
                    TextField(
                      controller: _oldPassword,
                      obscureText: true,
                      decoration: const InputDecoration(labelText: '旧密码'),
                    ),
                    TextField(
                      controller: _newPassword,
                      obscureText: true,
                      decoration: const InputDecoration(
                          labelText: '新密码', hintText: '8~20 位，含字母与数字'),
                    ),
                    TextField(
                      controller: _confirmPassword,
                      obscureText: true,
                      decoration: const InputDecoration(labelText: '确认新密码'),
                    ),
                    const SizedBox(height: 8),
                    Align(
                      alignment: Alignment.centerRight,
                      child: FilledButton(
                        onPressed: _busy ? null : _changePassword,
                        child: const Text('修改密码'),
                      ),
                    ),
                  ]),
                ),
              ),
              _sectionTitle(theme, '换绑手机号'),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Text('需旧手机号与新手机号双向短信验证；短信服务未接入时请联系管理员。',
                          style: theme.textTheme.bodySmall),
                      TextField(
                        controller: _oldCode,
                        keyboardType: TextInputType.number,
                        decoration: const InputDecoration(labelText: '旧手机号验证码'),
                      ),
                      TextField(
                        controller: _newPhone,
                        keyboardType: TextInputType.phone,
                        decoration: const InputDecoration(labelText: '新手机号'),
                      ),
                      TextField(
                        controller: _newCode,
                        keyboardType: TextInputType.number,
                        decoration: const InputDecoration(labelText: '新手机号验证码'),
                      ),
                      const SizedBox(height: 8),
                      Align(
                        alignment: Alignment.centerRight,
                        child: OutlinedButton(
                          onPressed: _busy ? null : _rebindPhone,
                          child: const Text('确认换绑'),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              _sectionTitle(theme, '登录设备'),
              Card(
                child: Column(
                  children: <Widget>[
                    FutureBuilder<List<Map<String, dynamic>>>(
                      future: _devices,
                      builder: (BuildContext context,
                          AsyncSnapshot<List<Map<String, dynamic>>> ds) {
                        if (ds.connectionState == ConnectionState.waiting) {
                          return const Padding(
                            padding: EdgeInsets.all(16),
                            child: Center(child: CircularProgressIndicator()),
                          );
                        }
                        if (ds.hasError) {
                          return const Padding(
                            padding: EdgeInsets.all(16),
                            child: Text('设备列表加载失败'),
                          );
                        }
                        final items = ds.data ?? const <Map<String, dynamic>>[];
                        if (items.isEmpty) {
                          return const Padding(
                            padding: EdgeInsets.all(16),
                            child: Text('暂无登录设备'),
                          );
                        }
                        return Column(
                          children: <Widget>[
                            for (final item in items)
                              ListTile(
                                leading: const Icon(Icons.devices),
                                title: Text(item['device'] as String? ?? '未知设备'),
                                subtitle: Text(
                                    '${item['ip'] ?? '-'} · ${_fmtTime(item['issued_at'] as String?)}'
                                    '${item['current'] == true ? ' · 当前设备' : ''}'),
                                trailing: item['current'] == true
                                    ? null
                                    : TextButton(
                                        onPressed: () => _revokeDevice(
                                            (item['id'] as num).toInt()),
                                        child: const Text('退出'),
                                      ),
                              ),
                          ],
                        );
                      },
                    ),
                    const Divider(height: 1),
                    Align(
                      alignment: Alignment.centerRight,
                      child: TextButton(
                        onPressed: _logoutAll,
                        child: const Text('退出全部设备'),
                      ),
                    ),
                  ],
                ),
              ),
              _sectionTitle(theme, '注销账号'),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Text('证书台账依法留存，身份信息匿名化；原证书仍可按编号验真。',
                          style: theme.textTheme.bodySmall),
                      const SizedBox(height: 8),
                      Align(
                        alignment: Alignment.centerRight,
                        child: OutlinedButton(
                          onPressed: _deleteAccount,
                          style: OutlinedButton.styleFrom(
                              foregroundColor: theme.colorScheme.error),
                          child: const Text('注销账号'),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  Widget _sectionTitle(ThemeData theme, String title) => Padding(
        padding: const EdgeInsets.fromLTRB(4, 16, 4, 6),
        child: Text(title, style: theme.textTheme.titleSmall),
      );

  String _fmtTime(String? raw) {
    if (raw == null || raw.isEmpty) return '-';
    final parsed = DateTime.tryParse(raw);
    if (parsed == null) return raw;
    final local = parsed.toLocal();
    return '${local.year}-${local.month.toString().padLeft(2, '0')}-${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }
}
