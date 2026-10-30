import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/my_course.dart';

/// 我的课程（PRD §7.13 L-1）：全部 / 学习中 / 已完成 / 未解锁，按最近学习排序。
class MyCoursesPage extends StatefulWidget {
  const MyCoursesPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<MyCoursesPage> createState() => _MyCoursesPageState();
}

class _MyCoursesPageState extends State<MyCoursesPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<MyCourse> _items = <MyCourse>[];
  bool _loading = true;
  bool _login = false;
  bool _failed = false;
  String _status = 'all';
  int? _major;

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
      final items = await _api.myCourses();
      if (!mounted) return;
      setState(() {
        _items = items;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _login = e is DioException && e.response?.statusCode == 401;
        _failed = true;
      });
    }
  }

  Future<void> _open(MyCourse course) async {
    final code = Uri.encodeComponent(course.courseCode);
    if (course.unlocked && course.lectureCount > 0) {
      await context.push('/learn/$code/${course.lastLectureNo ?? 1}');
    } else {
      await context.push('/courses/$code');
    }
    if (mounted) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final majors = <int, String>{};
    for (final c in _items) {
      if (c.majorId != null) majors[c.majorId!] = c.majorName ?? '未命名专业';
    }
    final filtered = _items
        .where((c) =>
            (_major == null || c.majorId == _major) &&
            (_status == 'all' || c.status == _status))
        .toList();
    return Scaffold(
      appBar: AppBar(title: const Text('我的课程')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _failed
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      Text(_login ? '请先登录后查看我的课程' : '课程加载失败，请重试'),
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
                            'learning': '学习中',
                            'completed': '已完成',
                            'locked': '未解锁',
                          }.entries)
                            Padding(
                              padding: const EdgeInsets.only(right: 8),
                              child: ChoiceChip(
                                label: Text(entry.value),
                                selected: _status == entry.key,
                                onSelected: (_) =>
                                    setState(() => _status = entry.key),
                              ),
                            ),
                        ],
                      ),
                    ),
                    if (majors.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        child: DropdownButton<int>(
                          isExpanded: true,
                          value: _major,
                          hint: const Text('全部专业'),
                          items: <DropdownMenuItem<int>>[
                            const DropdownMenuItem<int>(
                                value: null, child: Text('全部专业')),
                            for (final m in majors.entries)
                              DropdownMenuItem<int>(
                                  value: m.key, child: Text(m.value)),
                          ],
                          onChanged: (value) =>
                              setState(() => _major = value),
                        ),
                      ),
                    Expanded(
                      child: RefreshIndicator(
                        onRefresh: _load,
                        child: ListView(
                          physics: const AlwaysScrollableScrollPhysics(),
                          padding: const EdgeInsets.all(12),
                          children: filtered.isEmpty
                              ? <Widget>[
                                  const Padding(
                                    padding: EdgeInsets.all(40),
                                    child:
                                        Center(child: Text('暂无符合条件的课程')),
                                  ),
                                ]
                              : <Widget>[
                                  for (final unlocked in <bool>[true, false]) ...[
                                    if (filtered
                                        .any((c) => c.unlocked == unlocked))
                                      Padding(
                                        padding:
                                            const EdgeInsets.symmetric(
                                                vertical: 8),
                                        child: Text(unlocked ? '已解锁课程' : '未解锁课程'),
                                      ),
                                    for (final c in filtered.where(
                                        (c) => c.unlocked == unlocked))
                                      Card(
                                        child: ListTile(
                                          title: Text(c.name),
                                          subtitle: Column(
                                            crossAxisAlignment:
                                                CrossAxisAlignment.start,
                                            children: <Widget>[
                                              Text(
                                                  '${c.learnedLectures} / ${c.lectureCount} 讲 · ${c.progressPercent}%'),
                                              LinearProgressIndicator(
                                                  value: c.progressPercent
                                                          .clamp(0, 100) /
                                                      100),
                                              Text(!c.unlocked
                                                  ? '查看课程并解锁'
                                                  : c.completed
                                                      ? '已完成 · 再次学习'
                                                      : '继续学习'),
                                            ],
                                          ),
                                          trailing:
                                              const Icon(Icons.chevron_right),
                                          onTap: () => _open(c),
                                        ),
                                      ),
                                  ],
                                ],
                        ),
                      ),
                    ),
                  ],
                ),
    );
  }
}
