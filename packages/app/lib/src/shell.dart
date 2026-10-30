import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'api/api_client.dart';

/// 学员端底部导航（首页 / 课程 / 我的）。
///
/// 「我的」带未读消息角标；切换分支时刷新，避免角标长期停在旧值。
class AppShell extends StatefulWidget {
  const AppShell({super.key, required this.navigationShell, this.api});

  final StatefulNavigationShell navigationShell;
  final ApiClient? api;

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
  late final ApiClient _api = widget.api ?? ApiClient();
  int _unread = 0;

  @override
  void initState() {
    super.initState();
    _refreshUnread();
  }

  Future<void> _refreshUnread() async {
    try {
      if (!await _api.hasSession()) return;
      final page = await _api.messages();
      if (!mounted) return;
      setState(() => _unread = page.unread);
    } catch (_) {
      // 未登录 / 网络异常时不显示角标。
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: widget.navigationShell,
      bottomNavigationBar: NavigationBar(
        selectedIndex: widget.navigationShell.currentIndex,
        onDestinationSelected: (int i) {
          widget.navigationShell.goBranch(
            i,
            initialLocation: i == widget.navigationShell.currentIndex,
          );
          if (i == 2) _refreshUnread();
        },
        destinations: <NavigationDestination>[
          const NavigationDestination(
              icon: Icon(Icons.home_outlined),
              selectedIcon: Icon(Icons.home),
              label: '首页'),
          const NavigationDestination(
              icon: Icon(Icons.menu_book_outlined),
              selectedIcon: Icon(Icons.menu_book),
              label: '课程'),
          NavigationDestination(
            icon: Badge(
              isLabelVisible: _unread > 0,
              label: Text('$_unread'),
              child: const Icon(Icons.person_outline),
            ),
            selectedIcon: Badge(
              isLabelVisible: _unread > 0,
              label: Text('$_unread'),
              child: const Icon(Icons.person),
            ),
            label: '我的',
          ),
        ],
      ),
    );
  }
}
