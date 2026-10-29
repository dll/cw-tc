import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/my_course.dart';

/// 我的课程（PRD §7.13 L-1）：学习中 / 已完成，按最近学习时间排序。
class MyCoursesPage extends StatefulWidget {
  const MyCoursesPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<MyCoursesPage> createState() => _MyCoursesPageState();
}

class _MyCoursesPageState extends State<MyCoursesPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<List<MyCourse>> _future = _api.myCourses();

  void _reload() {
    setState(() => _future = _api.myCourses());
  }

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 2,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('我的课程'),
          bottom: const TabBar(tabs: <Tab>[Tab(text: '学习中'), Tab(text: '已完成')]),
        ),
        body: FutureBuilder<List<MyCourse>>(
          future: _future,
          builder: (BuildContext context, AsyncSnapshot<List<MyCourse>> snap) {
            if (snap.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snap.hasError) {
              return Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    const Text('我的课程加载失败，请先登录'),
                    const SizedBox(height: 8),
                    FilledButton(onPressed: _reload, child: const Text('重试')),
                  ],
                ),
              );
            }
            final items = snap.data ?? const <MyCourse>[];
            final learning = items.where((c) => !c.completed).toList();
            final completed = items.where((c) => c.completed).toList();
            return TabBarView(
              children: <Widget>[
                _courseList(learning, '暂无学习中的课程'),
                _courseList(completed, '暂无已完成的课程'),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _courseList(List<MyCourse> items, String emptyText) {
    if (items.isEmpty) {
      return Center(child: Text(emptyText));
    }
    return RefreshIndicator(
      onRefresh: () async {
        _reload();
        await _future;
      },
      child: ListView.separated(
        padding: const EdgeInsets.all(12),
        itemCount: items.length,
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (BuildContext context, int index) =>
            _courseCard(context, items[index]),
      ),
    );
  }

  Widget _courseCard(BuildContext context, MyCourse course) {
    final theme = Theme.of(context);
    return Card(
      child: InkWell(
        onTap: () =>
            context.push('/learn/${course.courseCode}/${course.lastLectureNo}'),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                children: <Widget>[
                  Expanded(
                    child: Text(course.name,
                        style: theme.textTheme.titleMedium),
                  ),
                  Text('${course.percent}%',
                      style: theme.textTheme.titleSmall
                          ?.copyWith(color: theme.colorScheme.primary)),
                ],
              ),
              if (course.majorName != null) ...<Widget>[
                const SizedBox(height: 2),
                Text(course.majorName!,
                    style: theme.textTheme.bodySmall
                        ?.copyWith(color: theme.colorScheme.primary)),
              ],
              const SizedBox(height: 8),
              LinearProgressIndicator(
                value: (course.percent.clamp(0, 100)) / 100,
                minHeight: 6,
                borderRadius: BorderRadius.circular(3),
              ),
              const SizedBox(height: 6),
              Row(
                children: <Widget>[
                  Text(
                    '已学 ${course.completedLectures}/${course.lectureCount} 讲',
                    style: theme.textTheme.bodySmall,
                  ),
                  const Spacer(),
                  Text(course.completed ? '复习' : '继续学习',
                      style: theme.textTheme.bodySmall
                          ?.copyWith(color: theme.colorScheme.primary)),
                  const Icon(Icons.chevron_right, size: 18),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
