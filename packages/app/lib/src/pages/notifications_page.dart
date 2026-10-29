import 'package:flutter/material.dart';

import '../api/api_client.dart';

/// 站内消息：管理员授权密码重置等系统通知（不依赖站外渠道）。
class NotificationsPage extends StatefulWidget {
  const NotificationsPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<NotificationsPage> createState() => _NotificationsPageState();
}

class _NotificationsPageState extends State<NotificationsPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<List<Map<String, dynamic>>> _future = _api.notifications();

  Future<void> _refresh() async {
    final next = _api.notifications();
    setState(() => _future = next);
    await next;
  }

  Future<void> _open(Map<String, dynamic> item) async {
    final id = (item['id'] as num?)?.toInt();
    if (id != null) {
      try {
        await _api.markNotificationRead(id);
      } catch (_) {
        // 已读失败不影响查看内容。
      }
    }
    if (!mounted) return;
    showDialog<void>(
      context: context,
      builder: (BuildContext dialogContext) => AlertDialog(
        title: Text(item['title'] as String? ?? '消息'),
        content: SelectableText(item['body'] as String? ?? ''),
        actions: <Widget>[
          TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: const Text('关闭')),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('消息')),
      body: FutureBuilder<List<Map<String, dynamic>>>(
        future: _future,
        builder: (BuildContext context,
            AsyncSnapshot<List<Map<String, dynamic>>> snap) {
          if (snap.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snap.hasError) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  const Text('消息加载失败'),
                  const SizedBox(height: 8),
                  FilledButton(
                      onPressed: _refresh, child: const Text('重试')),
                ],
              ),
            );
          }
          final items = snap.data ?? const <Map<String, dynamic>>[];
          if (items.isEmpty) {
            return const Center(child: Text('暂无消息'));
          }
          return RefreshIndicator(
            onRefresh: _refresh,
            child: ListView.separated(
              itemCount: items.length,
              separatorBuilder: (_, __) => const Divider(height: 1),
              itemBuilder: (BuildContext context, int index) {
                final item = items[index];
                final read = item['read'] == true;
                return ListTile(
                  leading: Icon(read
                      ? Icons.mark_email_read_outlined
                      : Icons.mark_email_unread_outlined),
                  title: Text(item['title'] as String? ?? '消息'),
                  subtitle: Text(
                    item['body'] as String? ?? '',
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(item),
                );
              },
            ),
          );
        },
      ),
    );
  }
}
