import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';

/// 资源阅读页：拉取文本类资源（讲义 Markdown / 代码 / HTML）原文并端内展示。
///
/// 二进制资源（课件 PPTX/PDF、视频、音频）不进本页——它们需对象存储/网关落地后
/// 在系统浏览器或播放器打开，LearnPage 侧用 SnackBar 提示。
class ResourceViewerPage extends StatefulWidget {
  const ResourceViewerPage({super.key, required this.resource, this.api});

  final LectureResource resource;
  final ApiClient? api;

  @override
  State<ResourceViewerPage> createState() => _ResourceViewerPageState();
}

class _ResourceViewerPageState extends State<ResourceViewerPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<String> _future;

  @override
  void initState() {
    super.initState();
    _future = _api.fetchResourceText(widget.resource);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.resource.fileName)),
      body: FutureBuilder<String>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<String> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  const Text('资源加载失败'),
                  const SizedBox(height: 8),
                  FilledButton(
                    onPressed: () => setState(() {
                      _future = _api.fetchResourceText(widget.resource);
                    }),
                    child: const Text('重试'),
                  ),
                ],
              ),
            );
          }
          final content = snapshot.data ?? '';
          return SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: SelectableText(
              content,
              style: const TextStyle(fontSize: 15, height: 1.6),
            ),
          );
        },
      ),
    );
  }
}
