import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';
import 'resource_viewer_page.dart';

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
                  title: Text(r.fileName),
                  subtitle: Text(_labelFor(r.kind) +
                      (r.sizeLabel.isEmpty ? '' : ' · ${r.sizeLabel}')),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => _open(context, r),
                ),
              );
            },
          );
        },
      ),
    );
  }

  void _open(BuildContext context, LectureResource r) {
    if (r.isTextKind) {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => ResourceViewerPage(resource: r, api: _api),
        ),
      );
      return;
    }
    // 二进制资源（课件/视频/音频）需对象存储与资源网关，暂未落地。
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('该资源为二进制文件，端内播放待资源网关落地后开放')),
    );
  }

  IconData _iconFor(String kind) {
    switch (kind) {
      case 'video':
        return Icons.play_circle_outline;
      case 'audio':
        return Icons.audiotrack_outlined;
      case 'slide':
        return Icons.slideshow_outlined;
      case 'markdown':
        return Icons.description_outlined;
      case 'code':
        return Icons.code;
      case 'html':
        return Icons.web_outlined;
      default:
        return Icons.insert_drive_file_outlined;
    }
  }

  String _labelFor(String kind) {
    switch (kind) {
      case 'video':
        return '视频';
      case 'audio':
        return '音频';
      case 'slide':
        return '课件';
      case 'markdown':
        return '讲义';
      case 'code':
        return '代码';
      case 'html':
        return 'HTML 页';
      default:
        return kind;
    }
  }
}
