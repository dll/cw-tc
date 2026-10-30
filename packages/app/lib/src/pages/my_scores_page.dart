import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/score.dart';

/// 我的成绩（PRD §7.13 L-3）：按课程分组，历次记录不覆盖、失败也展示。
class MyScoresPage extends StatefulWidget {
  const MyScoresPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<MyScoresPage> createState() => _MyScoresPageState();
}

class _MyScoresPageState extends State<MyScoresPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<Score> _items = <Score>[];
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
      final items = await _api.myScores();
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

  @override
  Widget build(BuildContext context) {
    final grouped = <String, List<Score>>{};
    for (final item in _items) {
      grouped.putIfAbsent(item.courseCode, () => <Score>[]).add(item);
    }
    return Scaffold(
      appBar: AppBar(title: const Text('我的成绩')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _failed
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      Text(_login ? '请先登录后查看成绩' : '成绩加载失败，请重试'),
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
              : grouped.isEmpty
                  ? const Center(child: Text('暂无成绩，完成测评后可查看'))
                  : RefreshIndicator(
                      onRefresh: _load,
                      child: ListView(
                        padding: const EdgeInsets.all(12),
                        children: <Widget>[
                          for (final entry in grouped.entries)
                            _courseGroup(entry.key, entry.value),
                        ],
                      ),
                    ),
    );
  }

  Widget _courseGroup(String courseCode, List<Score> attempts) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text(attempts.first.courseName, style: theme.textTheme.titleMedium),
            Text(courseCode, style: theme.textTheme.bodySmall),
            const Divider(height: 20),
            for (var i = 0; i < attempts.length; i++) ...<Widget>[
              _attemptRow(attempts[i], attempts.length - i),
              if (i != attempts.length - 1) const Divider(height: 18),
            ],
          ],
        ),
      ),
    );
  }

  Widget _attemptRow(Score score, int seq) {
    final theme = Theme.of(context);
    final color = score.isPass ? theme.colorScheme.primary : Colors.red.shade700;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        CircleAvatar(
          radius: 16,
          backgroundColor: color.withOpacity(0.12),
          child: Icon(score.isPass ? Icons.check : Icons.close,
              size: 18, color: color),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                '第 $seq 次 · ${score.levelLabel} · ${score.totalScore.toStringAsFixed(1)} 分',
                style: theme.textTheme.bodyMedium,
              ),
              Text(
                '${score.isPass ? '达标' : '未达标'} · ${score.bandLabel} · ${_time(score.examAt)}',
                style: theme.textTheme.bodySmall,
              ),
              if (score.certNo.isNotEmpty)
                Text('证书编号：${score.certNo}',
                    style: theme.textTheme.bodySmall
                        ?.copyWith(color: theme.colorScheme.primary)),
            ],
          ),
        ),
      ],
    );
  }

  String _time(String value) {
    if (value.isEmpty) return '—';
    return value.replaceFirst('T', ' ').split('.').first;
  }
}
