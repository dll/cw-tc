import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/course.dart';
import '../models/lecture.dart';

/// 课程详情（PRD v2 §7.2 B-2）：基本信息 + 讲次列表。
///
/// 课程信息与讲次并行拉取；未解锁课程只看到试学讲次（`is_preview`），
/// 学习跳转由后续「学习页」承载（本页仅展示大纲与试学标记）。
class CourseDetailPage extends StatefulWidget {
  const CourseDetailPage({super.key, required this.courseCode, this.api});

  final String courseCode;
  final ApiClient? api;

  @override
  State<CourseDetailPage> createState() => _CourseDetailPageState();
}

class _CourseDetailPageState extends State<CourseDetailPage> {
  late final ApiClient _api = widget.api ?? ApiClient();

  @override
  void initState() {
    super.initState();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(widget.courseCode)),
      body: FutureBuilder<Course>(
        future: _api.courseDetail(widget.courseCode),
        builder: (BuildContext context, AsyncSnapshot<Course> snap) {
          if (snap.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snap.hasError || !snap.hasData) {
            return const Center(child: Text('课程加载失败'));
          }
          final course = snap.data!;
          return ListView(
            padding: const EdgeInsets.all(16),
            children: <Widget>[
              Text(course.name, style: theme.textTheme.headlineSmall),
              if (course.intro != null && course.intro!.isNotEmpty) ...<Widget>[
                const SizedBox(height: 8),
                Text(course.intro!, style: theme.textTheme.bodyMedium),
              ],
              const SizedBox(height: 12),
              Row(
                children: <Widget>[
                  Text(course.priceLabel, style: theme.textTheme.titleLarge?.copyWith(color: theme.colorScheme.primary)),
                  const Spacer(),
                  FilledButton(
                    onPressed: () => context.push('/learn/${widget.courseCode}/1'),
                    child: const Text('开始学习'),
                  ),
                ],
              ),
              const Divider(height: 32),
              Text('讲次大纲', style: theme.textTheme.titleMedium),
              const SizedBox(height: 8),
              FutureBuilder<List<Lecture>>(
                future: _api.lectures(widget.courseCode),
                builder: (BuildContext context, AsyncSnapshot<List<Lecture>> ls) {
                  if (ls.connectionState == ConnectionState.waiting) {
                    return const Padding(
                      padding: EdgeInsets.all(24),
                      child: Center(child: CircularProgressIndicator()),
                    );
                  }
                  final lectures = ls.data ?? const <Lecture>[];
                  if (lectures.isEmpty) {
                    return const Padding(
                      padding: EdgeInsets.all(24),
                      child: Center(child: Text('暂无讲次')),
                    );
                  }
                  return Column(
                    children: lectures.map((lecture) {
                      return ListTile(
                        dense: true,
                        leading: CircleAvatar(radius: 14, child: Text('${lecture.no}')),
                        title: Text(lecture.title),
                        trailing: lecture.isPreview ? const Chip(label: Text('试学')) : null,
                        onTap: () => context.push('/learn/${widget.courseCode}/${lecture.no}'),
                      );
                    }).toList(),
                  );
                },
              ),
            ],
          );
        },
      ),
    );
  }
}
