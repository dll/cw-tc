import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/favorite.dart';

/// 我的收藏（PRD §7.14 M-2）：登录后可收藏；下架课程保留并标注。
class MyFavoritesPage extends StatefulWidget {
  const MyFavoritesPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<MyFavoritesPage> createState() => _MyFavoritesPageState();
}

class _MyFavoritesPageState extends State<MyFavoritesPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<FavoriteCourse> _items = <FavoriteCourse>[];
  bool _loading = true;
  bool _failed = false;
  bool _login = false;

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
      final items = await _api.myFavorites();
      if (!mounted) return;
      setState(() {
        _items = items;
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

  Future<void> _remove(FavoriteCourse item) async {
    try {
      await _api.setFavorite(item.courseCode, false);
      if (mounted) await _load();
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('取消收藏失败，请稍后重试')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('我的收藏')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _failed
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      Text(_login ? '请先登录后查看收藏' : '收藏加载失败，请重试'),
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
              : _items.isEmpty
                  ? const Center(child: Text('暂无收藏，去课程广场逛逛'))
                  : RefreshIndicator(
                      onRefresh: _load,
                      child: ListView.separated(
                        padding: const EdgeInsets.all(12),
                        itemCount: _items.length,
                        separatorBuilder: (_, __) => const SizedBox(height: 8),
                        itemBuilder: (BuildContext context, int i) =>
                            _card(_items[i]),
                      ),
                    ),
    );
  }

  Widget _card(FavoriteCourse item) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      child: ListTile(
        title: Text(item.name),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            if (item.majorName != null) Text(item.majorName!),
            Text('${item.priceLabel}${item.unlocked ? ' · 已解锁' : ''}'),
            if (item.offline)
              Text('已下架，仍保留收藏',
                  style: TextStyle(color: theme.colorScheme.error)),
          ],
        ),
        trailing: IconButton(
          tooltip: '取消收藏',
          icon: const Icon(Icons.favorite),
          onPressed: () => _remove(item),
        ),
        onTap: () => context.push(
            '/courses/${Uri.encodeComponent(item.courseCode)}'),
      ),
    );
  }
}
