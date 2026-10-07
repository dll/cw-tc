import 'package:flutter/material.dart';

/// 个人中心（PRD v2 §8.1 `/me`）——基础骨架。
///
/// M1 起接入：登录态、我的课程 / 订单 / 成绩 / 证书真实数据。
class MePage extends StatelessWidget {
  const MePage({super.key});

  @override
  Widget build(BuildContext context) {
    final List<(IconData, String, String)> entries = <(IconData, String, String)>[
      (Icons.school_outlined, '我的课程', '学习中 / 已完成'),
      (Icons.receipt_long_outlined, '我的订单', '订单与发票'),
      (Icons.assignment_turned_in_outlined, '我的成绩', '测评记录与分数'),
      (Icons.verified_outlined, '我的证书', '查看 / 下载 / 验真'),
      (Icons.settings_outlined, '设置', '账号与安全'),
    ];
    return Scaffold(
      appBar: AppBar(title: const Text('我的')),
      body: Column(
        children: <Widget>[
          const ListTile(
            leading: CircleAvatar(child: Icon(Icons.person)),
            title: Text('未登录'),
            subtitle: Text('手机号注册 / 登录'),
          ),
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
                  onTap: () {},
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
