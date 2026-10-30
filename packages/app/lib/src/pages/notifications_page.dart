import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/message.dart';

/// 消息中心（PRD §7.13 L-5）：系统 / 事务 / 广播、已读未读、全部已读、广播退订。
class NotificationsPage extends StatefulWidget {
  const NotificationsPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<NotificationsPage> createState() => _NotificationsPageState();
}

class _NotificationsPageState extends State<NotificationsPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<AppMessage> _items = <AppMessage>[];
  MessagePrefs _prefs = const MessagePrefs(broadcastOptOut: false);
  int _unread = 0;
  bool _loading = true;
  bool _failed = false;
  bool _login = false;
  bool _unreadOnly = false;
  String _filter = 'all';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _failed = false;
      _login = false;
    });
    try {
      final page = await _api.messages();
      if (!mounted) return;
      setState(() {
        _items = page.items;
        _unread = page.unread;
        _prefs = page.prefs;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _failed = true;
        _login = e is DioException && e.response?.statusCode == 401;
      });
    }
  }

  Future<void> _open(AppMessage item) async {
    if (!item.read) {
      try {
        await _api.markNotificationRead(item.id);
      } catch (_) {
        // 已读失败不影响查看内容。
      }
    }
    if (!mounted) return;
    final go = await showDialog<bool>(
      context: context,
      builder: (BuildContext dialogContext) => AlertDialog(
        title: Text(item.title),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(item.categoryLabel,
                  style: Theme.of(dialogContext).textTheme.bodySmall),
              const SizedBox(height: 8),
              SelectableText(item.body),
            ],
          ),
        ),
        actions: <Widget>[
          TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: const Text('关闭')),
          if (item.link.startsWith('/'))
            FilledButton(
                onPressed: () => Navigator.of(dialogContext).pop(true),
                child: const Text('查看相关页面')),
        ],
      ),
    );
    if (!mounted) return;
    await _load();
    if (go == true && mounted) {
      await context.push(item.link);
    }
  }

  Future<void> _markAllRead() async {
    try {
      await _api.markAllMessagesRead();
      if (!mounted) return;
      await _load();
    } catch (e) {
      if (mounted) _snack(_errorMessage(e));
    }
  }

  Future<void> _setBroadcast(bool receive) async {
    try {
      await _api.setBroadcastOptOut(!receive);
      if (!mounted) return;
      await _load();
    } catch (e) {
      if (mounted) _snack(_errorMessage(e));
    }
  }

  void _snack(String message) {
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(message)));
  }

  String _errorMessage(Object error) {
    if (error is DioException) {
      final data = error.response?.data;
      if (data is Map<String, dynamic> && data['msg'] is String) {
        return data['msg'] as String;
      }
    }
    final text = error.toString();
    return text.startsWith('Exception: ')
        ? text.substring('Exception: '.length)
        : '操作失败，请稍后重试';
  }

  @override
  Widget build(BuildContext context) {
    final filtered = _items.where((m) {
      if (_unreadOnly && m.read) return false;
      return _filter == 'all' || m.category == _filter;
    }).toList();
    return Scaffold(
      appBar: AppBar(
        title: Text(_unread > 0 ? '消息（$_unread）' : '消息'),
        actions: <Widget>[
          if (_unread > 0)
            TextButton(
              onPressed: _markAllRead,
              child: const Text('全部已读'),
            ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _failed
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      Text(_login ? '请先登录后查看消息' : '消息加载失败'),
                      TextButton(
                        onPressed: () async {
                          if (_login) {
                            await context.push('/login');
                            if (!mounted) return;
                          }
                          await _load();
                        },
                        child: Text(_login ? '去登录' : '重试'),
                      ),
                    ],
                  ),
                )
              : Column(
                  children: <Widget>[
                    SingleChildScrollView(
                      scrollDirection: Axis.horizontal,
                      padding: const EdgeInsets.all(12),
                      child: Row(
                        children: <Widget>[
                          for (final entry in <String, String>{
                            'all': '全部',
                            'system': '系统',
                            'transaction': '事务',
                            'broadcast': '广播',
                          }.entries)
                            Padding(
                              padding: const EdgeInsets.only(right: 8),
                              child: ChoiceChip(
                                label: Text(entry.value),
                                selected: _filter == entry.key,
                                onSelected: (_) =>
                                    setState(() => _filter = entry.key),
                              ),
                            ),
                          const SizedBox(width: 8),
                          FilterChip(
                            label: const Text('只看未读'),
                            selected: _unreadOnly,
                            onSelected: (v) =>
                                setState(() => _unreadOnly = v),
                          ),
                        ],
                      ),
                    ),
                    SwitchListTile(
                      dense: true,
                      title: const Text('接收广播消息'),
                      subtitle: const Text('系统消息与事务通知不可退订'),
                      value: !_prefs.broadcastOptOut,
                      onChanged: _setBroadcast,
                    ),
                    const Divider(height: 1),
                    Expanded(
                      child: RefreshIndicator(
                        onRefresh: _load,
                        child: ListView.separated(
                          physics: const AlwaysScrollableScrollPhysics(),
                          itemCount: filtered.isEmpty ? 1 : filtered.length,
                          separatorBuilder: (_, __) =>
                              const Divider(height: 1),
                          itemBuilder: (BuildContext context, int index) {
                            if (filtered.isEmpty) {
                              return const Padding(
                                padding: EdgeInsets.all(40),
                                child: Center(child: Text('暂无消息')),
                              );
                            }
                            return _messageTile(filtered[index]);
                          },
                        ),
                      ),
                    ),
                  ],
                ),
    );
  }

  Widget _messageTile(AppMessage item) {
    final theme = Theme.of(context);
    return ListTile(
      leading: Icon(
        item.read ? Icons.mark_email_read_outlined : Icons.mark_email_unread,
        color: item.read ? theme.disabledColor : theme.colorScheme.primary,
      ),
      title: Row(
        children: <Widget>[
          Expanded(
            child: Text(
              item.title,
              style: TextStyle(
                  fontWeight: item.read ? FontWeight.normal : FontWeight.w600),
            ),
          ),
          if (!item.read)
            Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(
                  color: theme.colorScheme.primary, shape: BoxShape.circle),
            ),
        ],
      ),
      subtitle: Text(
        '${item.categoryLabel} · ${item.body}',
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
      ),
      trailing: const Icon(Icons.chevron_right),
      onTap: () => _open(item),
    );
  }
}
