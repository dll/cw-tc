import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';

/// 个人中心（PRD v2 §8.1 `/me`）：登录态 + 我的课程 / 订单 / 成绩 / 证书入口。
class MePage extends StatefulWidget {
  const MePage({super.key, this.api});

  final ApiClient? api;

  @override
  State<MePage> createState() => _MePageState();
}

class _MePageState extends State<MePage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  bool _loggedIn = false;
  String? _phone;

  @override
  void initState() {
    super.initState();
    _refresh();
  }

  Future<void> _refresh() async {
    try {
      final loggedIn = await _api.hasSession();
      final phone = await _api.storedPhone();
      if (!mounted) return;
      setState(() {
        _loggedIn = loggedIn;
        _phone = phone;
      });
    } catch (_) {
      // 安全存储不可用（如测试环境）按未登录处理。
    }
  }

  Future<void> _logout() async {
    try {
      await _api.logout();
    } catch (_) {
      // 忽略存储异常。
    }
    if (!mounted) return;
    setState(() {
      _loggedIn = false;
      _phone = null;
    });
  }

  String get _maskedPhone {
    final p = _phone ?? '';
    if (p.length != 11) return p;
    return '${p.substring(0, 3)}****${p.substring(7)}';
  }

  @override
  Widget build(BuildContext context) {
    final List<(IconData, String, String)> entries = <(IconData, String, String)>[
      (Icons.school_outlined, '我的课程', '学习中 / 已完成'),
      (Icons.receipt_long_outlined, '我的订单', '订单与发票'),
      (Icons.assignment_turned_in_outlined, '我的成绩', '测评记录与分数'),
      (Icons.verified_outlined, '我的证书', '查看 / 下载 / 验真'),
      (Icons.notifications_outlined, '消息', '系统通知 / 重置码'),
      (Icons.settings_outlined, '设置', '账号与安全'),
    ];
    return Scaffold(
      appBar: AppBar(title: const Text('我的')),
      body: Column(
        children: <Widget>[
          ListTile(
            leading: CircleAvatar(
              child: Icon(_loggedIn ? Icons.person : Icons.person_outline),
            ),
            title: Text(_loggedIn
                ? ((_phone == null || _phone!.isEmpty) ? '已登录' : _maskedPhone)
                : '未登录'),
            subtitle: Text(_loggedIn ? '已登录' : '手机号注册 / 登录'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () async {
              if (_loggedIn) return;
              await context.push('/login');
              await _refresh();
            },
          ),
          if (_loggedIn) ...<Widget>[
            ListTile(
              leading: const Icon(Icons.logout),
              title: const Text('退出登录'),
              onTap: _logout,
            ),
          ],
          const Divider(height: 1),
          Expanded(
            child: ListView.separated(
              itemCount: entries.length,
              separatorBuilder: (_, __) => const Divider(height: 1),
              itemBuilder: (BuildContext context, int i) {
                final (IconData icon, String title, String sub) = entries[i];
                return ListTile(
                  leading: Icon(icon),
                  title: Text(title),
                  subtitle: Text(sub),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () {
                    if (title == '我的课程') context.push('/me/courses');
                    if (title == '我的订单') context.push('/me/orders');
                    if (title == '我的证书') context.push('/certs');
                    if (title == '消息') context.push('/notifications');
                    if (title == '设置') context.push('/settings');
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
