import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';

/// 学习页（PRD v2 §7.4 B-4）：某讲的资源清单（讲义 / 课件 / 视频）。
///
/// 资源带短时签名 URL；本页展示清单（类型/标题/大小），
/// 视频播放 / 讲义在线阅读接续资源网关后落地。
class LearnPage extends StatefulWidget {
  const LearnPage({super.key, required this.courseCode, required this.no, this.api});

  final String courseCode;
  final int no;
  final ApiClient? api;

  @override
  State<LearnPage> createState() => _LearnPageState();
}

class _LearnPageState extends State<LearnPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<List<LectureResource>> _future;

  @override
  void initState() {
    super.initState();
    _future = _api.lectureResources(widget.courseCode, widget.no);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('第 ${widget.no} 讲')),
      body: FutureBuilder<List<LectureResource>>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<List<LectureResource>> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return const Center(child: Text('资源加载失败'));
          }
          final resources = snapshot.data ?? const <LectureResource>[];
          if (resources.isEmpty) {
            return const Center(child: Text('本讲暂无资源'));
          }
          return ListView.separated(
            padding: const EdgeInsets.all(12),
            itemCount: resources.length,
            separatorBuilder: (_, __) => const SizedBox(height: 8),
            itemBuilder: (BuildContext context, int i) {
              final r = resources[i];
              return Card(
                margin: EdgeInsets.zero,
                child: ListTile(
                  leading: Icon(_iconFor(r.kind)),
                  title: Text(r.title),
                  subtitle: Text('${r.kind}${r.sizeLabel.isEmpty ? '' : ' · ${r.sizeLabel}'}'),
                ),
              );
            },
          );
        },
      ),
    );
  }

  IconData _iconFor(String kind) {
    switch (kind) {
      case 'video':
        return Icons.play_circle_outline;
      case 'slides':
        return Icons.slideshow_outlined;
      case 'handout':
      case '讲义':
        return Icons.description_outlined;
      default:
        return Icons.insert_drive_file_outlined;
    }
  }
}
